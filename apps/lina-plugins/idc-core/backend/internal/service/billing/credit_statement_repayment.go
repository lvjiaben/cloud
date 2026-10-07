package billing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditPrincipalRepaymentPayload struct {
	ExposureNo      string `json:"exposure_no"`
	AmountMinor     int64  `json:"amount_minor"`
	ExposureVersion int64  `json:"exposure_version"`
}

func (s *serviceImpl) ApplyConfirmedCreditStatementRepayment(
	ctx context.Context,
	input CreditStatementRepaymentInput,
) (*CreditRepayment, error) {
	normalized, err := normalizeCreditStatementRepayment(input)
	if err != nil {
		return nil, err
	}
	var result *CreditRepayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		account, statement, replay, loadErr := s.loadCreditStatementRepaymentRequest(
			ctx, normalized,
		)
		if loadErr != nil {
			return loadErr
		}
		if replay != nil {
			result = replay
			return nil
		}
		delinquency, loadErr := loadCreditDelinquencyByStatement(
			ctx, s.database, statement.StatementNo, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if (statement.StatusCode == "overdue") != (delinquency != nil) {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		if delinquency != nil {
			if _, _, loadErr = s.accrueCreditLateFeeLocked(
				ctx, account, statement, delinquency,
			); loadErr != nil {
				return loadErr
			}
		}
		feePayment, principalPayment, principalPayload, payloadErr :=
			s.prepareCreditStatementRepayment(ctx, normalized, account, statement)
		if payloadErr != nil {
			return payloadErr
		}
		repaymentNo := deterministicCreditNo(
			"crr_", "statement", normalized.FundingSourceType,
			normalized.FundingSourceReference,
		)
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment (
    tenant_id, repayment_no, credit_account_no, customer_no, currency_code,
    repayment_type_code, funding_source_type_code, funding_source_reference,
    ledger_transaction_no, amount_minor, allocated_amount_minor,
    unallocated_amount_minor, refunded_amount_minor, status_code,
    source_snapshot_digest, confirmation_evidence_digest, confirmed_at, version
) VALUES (0, ?, ?, ?, ?, 'statement', ?, ?, ?, ?, ?, 0, 0,
          'allocated', ?, ?, ?, 1)`, repaymentNo, account.CreditAccountNo,
			account.CustomerNo, account.CurrencyCode, normalized.FundingSourceType,
			normalized.FundingSourceReference, normalized.LedgerTransactionNo,
			normalized.AmountMinor, normalized.AmountMinor,
			normalized.SourceSnapshotDigest, normalized.ConfirmationEvidenceDigest,
			account.Clock); insertErr != nil {
			return gerror.Wrap(insertErr, "create confirmed credit statement repayment")
		}
		if feePayment > 0 {
			if allocationErr := appendCreditStatementRepaymentAllocation(
				ctx, s.database, repaymentNo, account, statement,
				"statement_fee", "", feePayment,
			); allocationErr != nil {
				return allocationErr
			}
		}
		if principalPayment > 0 {
			for _, principal := range principalPayload {
				if allocationErr := appendCreditStatementRepaymentAllocation(
					ctx, s.database, repaymentNo, account, statement,
					"statement_principal", principal.ExposureNo, principal.AmountMinor,
				); allocationErr != nil {
					return allocationErr
				}
			}
		}
		afterOutstanding := statement.OutstandingMinor - normalized.AmountMinor
		afterPaid := statement.PaidAmountMinor + normalized.AmountMinor
		afterStatus := "partially_paid"
		var settledAt any
		if afterOutstanding == 0 {
			afterStatus = "paid"
			settledAt = account.Clock
		} else if statement.StatusCode == "overdue" {
			afterStatus = "overdue"
		}
		updatedStatement, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET paid_amount_minor = ?, outstanding_amount_minor = ?, status_code = ?,
    settled_at = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND statement_no = ? AND version = ?`,
			afterPaid, afterOutstanding, afterStatus, settledAt,
			statement.StatementNo, statement.Version)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "apply credit statement repayment")
		}
		updatedCount, rowsErr := updatedStatement.RowsAffected()
		if rowsErr != nil || updatedCount != 1 {
			return bizerr.NewCode(CodeCreditConflict)
		}
		if delinquencyErr := s.applyCreditRepaymentToDelinquency(
			ctx, delinquency, account.Clock, feePayment, principalPayment,
			afterOutstanding,
		); delinquencyErr != nil {
			return delinquencyErr
		}
		if principalPayment > 0 {
			encodedPayload, encodeErr := json.Marshal(principalPayload)
			if encodeErr != nil {
				return gerror.Wrap(encodeErr, "encode principal repayment allocation")
			}
			updatedExposures, exposureErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure exposure
SET billed_principal_minor = exposure.billed_principal_minor - payment.amount_minor,
    repaid_principal_minor = exposure.repaid_principal_minor + payment.amount_minor,
    status_code = CASE
        WHEN exposure.billed_principal_minor - payment.amount_minor = 0
         AND exposure.unbilled_principal_minor = 0 THEN 'settled'
        ELSE 'partially_repaid'
    END,
    version = exposure.version + 1, updated_at = CURRENT_TIMESTAMP
FROM jsonb_to_recordset(?::jsonb) AS payment(
    exposure_no TEXT, amount_minor BIGINT, exposure_version BIGINT
)
WHERE exposure.tenant_id = 0 AND exposure.exposure_no = payment.exposure_no
  AND exposure.version = payment.exposure_version
  AND exposure.billed_principal_minor >= payment.amount_minor`, string(encodedPayload))
			if exposureErr != nil {
				return gerror.Wrap(exposureErr, "apply principal repayment to exposures")
			}
			updatedCount, rowsErr = updatedExposures.RowsAffected()
			if rowsErr != nil || updatedCount != int64(len(principalPayload)) {
				return bizerr.NewCode(CodeCreditConflict)
			}
			afterAvailable := account.AvailableLimitMinor + principalPayment
			afterBilled := account.BilledPrincipalMinor - principalPayment
			afterDelinquent := account.DelinquentPrincipalMinor
			if statement.StatusCode == "overdue" {
				afterDelinquent -= minCreditAmount(afterDelinquent, principalPayment)
			}
			updatedAccount, accountErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = ?, billed_principal_minor = ?,
    delinquent_principal_minor = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
				afterAvailable, afterBilled, afterDelinquent,
				account.CreditAccountNo, account.Version)
			if accountErr != nil {
				return gerror.Wrap(accountErr, "restore repaid credit limit")
			}
			updatedCount, rowsErr = updatedAccount.RowsAffected()
			if rowsErr != nil || updatedCount != 1 {
				return bizerr.NewCode(CodeCreditConflict)
			}
			if movementErr := s.appendCreditMovement(ctx, creditMovementInput{
				MovementNo: deterministicCreditNo("crm_", "repayment", repaymentNo),
				Account:    account, StatementNo: statement.StatementNo,
				RepaymentNo: repaymentNo, MovementType: "repayment",
				AmountMinor: principalPayment, StatusAfter: account.StatusCode,
				AvailableAfter: afterAvailable,
				UnbilledAfter:  account.UnbilledPrincipalMinor,
				BilledAfter:    afterBilled, DelinquentAfter: afterDelinquent,
				WrittenOffAfter: account.WrittenOffPrincipalMinor,
				SourceType:      "repayment", SourceReference: repaymentNo,
				EvidenceDigest: normalized.ConfirmationEvidenceDigest,
				ActorType:      "system", ActorID: 0, OccurredAt: account.Clock,
			}); movementErr != nil {
				return movementErr
			}
			account.AvailableLimitMinor = afterAvailable
			account.BilledPrincipalMinor = afterBilled
			account.DelinquentPrincipalMinor = afterDelinquent
			account.Version++
		}
		statement.PaidAmountMinor = afterPaid
		statement.OutstandingMinor = afterOutstanding
		statement.StatusCode = afterStatus
		statement.Version++
		created, loadErr := loadCreditRepaymentBySource(
			ctx, s.database, normalized.FundingSourceType,
			normalized.FundingSourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		if created == nil {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		result = projectStatementRepayment(created, statement, account, false)
		return nil
	})
	return creditRepaymentResult(result, err)
}

func (s *serviceImpl) loadCreditStatementRepaymentRequest(
	ctx context.Context,
	input CreditStatementRepaymentInput,
) (*creditAccountRow, *creditStatementRow, *CreditRepayment, error) {
	identity, err := loadCreditStatement(ctx, s.database, input.StatementNo)
	if err != nil {
		return nil, nil, nil, err
	}
	if identity == nil {
		return nil, nil, nil, bizerr.NewCode(CodeCreditNotFound)
	}
	account, err := loadCreditAccountByNo(ctx, s.database, identity.CreditAccountNo, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if account == nil || !validCreditAccountProjection(account) {
		return nil, nil, nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	existing, err := loadCreditRepaymentBySource(
		ctx, s.database, input.FundingSourceType, input.FundingSourceReference,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if existing != nil {
		return projectCreditStatementRepaymentReplay(input, existing, identity, account)
	}
	statement, err := loadCreditStatementForUpdate(ctx, s.database, input.StatementNo)
	if err != nil {
		return nil, nil, nil, err
	}
	if !validCreditStatementRepaymentTarget(input, statement, account) {
		return nil, nil, nil, bizerr.NewCode(CodeCreditConflict)
	}
	return account, statement, nil, nil
}

func projectCreditStatementRepaymentReplay(
	input CreditStatementRepaymentInput,
	existing *creditRepaymentRow,
	statement *creditStatementRow,
	account *creditAccountRow,
) (*creditAccountRow, *creditStatementRow, *CreditRepayment, error) {
	if existing.CreditAccountNo != account.CreditAccountNo ||
		existing.RepaymentType != "statement" || existing.StatementNo != input.StatementNo ||
		existing.LedgerTransactionNo != input.LedgerTransactionNo ||
		existing.AmountMinor != input.AmountMinor ||
		existing.SourceSnapshotDigest != input.SourceSnapshotDigest ||
		existing.ConfirmationEvidenceDigest != input.ConfirmationEvidenceDigest {
		return nil, nil, nil, bizerr.NewCode(CodeCreditConflict)
	}
	replay := projectStatementRepayment(existing, statement, account, true)
	if replay == nil {
		return nil, nil, nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return account, statement, replay, nil
}

func validCreditStatementRepaymentTarget(
	input CreditStatementRepaymentInput,
	statement *creditStatementRow,
	account *creditAccountRow,
) bool {
	if statement == nil || statement.CreditAccountNo != account.CreditAccountNo ||
		(statement.StatusCode != "open" && statement.StatusCode != "partially_paid" &&
			statement.StatusCode != "overdue") ||
		statement.WrittenOffMinor != 0 || statement.OutstandingMinor < input.AmountMinor {
		return false
	}
	return account.DelinquentPrincipalMinor == 0 || statement.StatusCode == "overdue"
}

func (s *serviceImpl) prepareCreditStatementRepayment(
	ctx context.Context,
	input CreditStatementRepaymentInput,
	account *creditAccountRow,
	statement *creditStatementRow,
) (int64, int64, []creditPrincipalRepaymentPayload, error) {
	if err := validateCreditRepaymentLedger(
		ctx, s.database, account, input.FundingSourceType,
		input.FundingSourceReference, input.LedgerTransactionNo, input.AmountMinor,
	); err != nil {
		return 0, 0, nil, err
	}
	var allocated struct {
		Principal int64 `orm:"principal_minor"`
		Fee       int64 `orm:"fee_minor"`
	}
	if err := s.database.GetScan(ctx, &allocated, `
SELECT
    COALESCE(SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
        FILTER (WHERE allocation_type_code = 'statement_principal'), 0) AS principal_minor,
    COALESCE(SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
        FILTER (WHERE allocation_type_code = 'statement_fee'), 0) AS fee_minor
FROM plugin_idc_core_credit_repayment_allocation
WHERE tenant_id = 0 AND statement_no = ?`, statement.StatementNo); err != nil {
		return 0, 0, nil, gerror.Wrap(err, "load existing credit statement allocations")
	}
	principalRemaining := statement.PrincipalMinor - allocated.Principal
	feeRemaining := statement.FeeMinor - allocated.Fee
	if principalRemaining < 0 || feeRemaining < 0 ||
		input.AmountMinor > principalRemaining+feeRemaining {
		return 0, 0, nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	feePayment := minCreditAmount(input.AmountMinor, feeRemaining)
	principalPayment := input.AmountMinor - feePayment
	if principalPayment > account.BilledPrincipalMinor {
		return 0, 0, nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	payload, err := lockCreditPrincipalRepaymentPayload(
		ctx, s.database, statement.StatementNo, principalPayment,
	)
	return feePayment, principalPayment, payload, err
}

func (s *serviceImpl) applyCreditRepaymentToDelinquency(
	ctx context.Context,
	caseRow *creditDelinquencyRow,
	clock time.Time,
	feePayment int64,
	principalPayment int64,
	afterOutstanding int64,
) error {
	if caseRow == nil {
		return nil
	}
	if caseRow.StatusCode != "grace_period" && caseRow.StatusCode != "frozen" &&
		caseRow.StatusCode != "service_action_pending" {
		return bizerr.NewCode(CodeCreditConflict)
	}
	if feePayment > caseRow.LateFeeMinor || principalPayment > caseRow.PrincipalMinor {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	afterPrincipal := caseRow.PrincipalMinor - principalPayment
	afterLateFee := caseRow.LateFeeMinor - feePayment
	if afterOutstanding > 0 && afterOutstanding != afterPrincipal+afterLateFee {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	afterStatus := caseRow.StatusCode
	var resolvedAt any
	if afterOutstanding == 0 {
		afterStatus = "resolved"
		resolvedAt = clock
	}
	caseRow.PrincipalMinor = afterPrincipal
	caseRow.LateFeeMinor = afterLateFee
	caseRow.OutstandingAmountMinor = afterOutstanding
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_delinquency_case
SET principal_minor = ?, late_fee_minor = ?, outstanding_amount_minor = ?,
    status_code = ?, resolved_at = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND version = ?`,
		caseRow.PrincipalMinor, caseRow.LateFeeMinor, caseRow.OutstandingAmountMinor,
		afterStatus, resolvedAt, caseRow.CaseNo, caseRow.Version)
	if err != nil {
		return gerror.Wrap(err, "apply repayment to credit delinquency")
	}
	if err = requireSingleCreditUpdate(updated); err != nil {
		return err
	}
	caseRow.StatusCode = afterStatus
	if afterOutstanding == 0 {
		resolved := clock
		caseRow.ResolvedAt = &resolved
	}
	caseRow.Version++
	return nil
}

func loadCreditStatementForUpdate(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
) (*creditStatementRow, error) {
	var row *creditStatementRow
	if err := database.GetScan(ctx, &row, `
SELECT statement_no, credit_account_no, customer_no, currency_code,
       terms_no, terms_version, period_started_at, period_ended_at,
       line_count, principal_minor, fee_minor, total_amount_minor,
       paid_amount_minor, written_off_amount_minor, outstanding_amount_minor,
       status_code, issued_at, payment_due_at, version
FROM plugin_idc_core_credit_statement
WHERE tenant_id = 0 AND statement_no = ? FOR UPDATE`, statementNo); err != nil {
		return nil, gerror.Wrap(err, "lock credit statement")
	}
	return row, nil
}

func lockCreditPrincipalRepaymentPayload(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
	amountMinor int64,
) ([]creditPrincipalRepaymentPayload, error) {
	if amountMinor == 0 {
		return nil, nil
	}
	rows := make([]struct {
		ExposureNo  string `orm:"exposure_no"`
		BilledMinor int64  `orm:"billed_principal_minor"`
		Version     int64  `orm:"version"`
	}, 0, 64)
	if err := database.GetScan(ctx, &rows, `
SELECT exposure.exposure_no, exposure.billed_principal_minor, exposure.version
FROM plugin_idc_core_credit_statement_line line
JOIN plugin_idc_core_credit_exposure exposure
  ON exposure.exposure_no = line.exposure_no
 AND exposure.credit_account_no = line.credit_account_no
 AND exposure.customer_no = line.customer_no
 AND exposure.currency_code = line.currency_code
WHERE line.tenant_id = 0 AND line.statement_no = ?
  AND line.line_type_code = 'principal'
  AND exposure.billed_principal_minor > 0
ORDER BY line.line_number ASC, line.id ASC
LIMIT ? FOR UPDATE OF exposure`, statementNo, maximumCreditStatementLines); err != nil {
		return nil, gerror.Wrap(err, "lock statement principal exposures")
	}
	remaining := amountMinor
	payload := make([]creditPrincipalRepaymentPayload, 0, len(rows))
	for _, row := range rows {
		if remaining == 0 {
			break
		}
		allocated := minCreditAmount(remaining, row.BilledMinor)
		payload = append(payload, creditPrincipalRepaymentPayload{
			ExposureNo: row.ExposureNo, AmountMinor: allocated,
			ExposureVersion: row.Version,
		})
		remaining -= allocated
	}
	if remaining != 0 || len(payload) == 0 {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return payload, nil
}

func appendCreditStatementRepaymentAllocation(
	ctx context.Context,
	database gdb.DB,
	repaymentNo string,
	account *creditAccountRow,
	statement *creditStatementRow,
	allocationType string,
	exposureNo string,
	amountMinor int64,
) error {
	allocationNo := deterministicCreditNo(
		"cal_", allocationType, repaymentNo, exposureNo,
	)
	digest, err := contentDigest(struct {
		AllocationNo, RepaymentNo, CreditAccountNo, StatementNo string
		AllocationType, ExposureNo                              string
		AmountMinor                                             int64
	}{
		allocationNo, repaymentNo, account.CreditAccountNo,
		statement.StatementNo, allocationType, exposureNo, amountMinor,
	})
	if err != nil {
		return err
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment_allocation (
    tenant_id, allocation_no, repayment_no, credit_account_no, customer_no,
    currency_code, statement_no, exposure_no, allocation_type_code,
    direction_code, amount_minor, source_reference, content_digest
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, 'apply', ?, ?, ?)`,
		allocationNo, repaymentNo, account.CreditAccountNo, account.CustomerNo,
		account.CurrencyCode, statement.StatementNo, refundNullableString(exposureNo), allocationType,
		amountMinor, allocationNo, digest); err != nil {
		return gerror.Wrap(err, "append credit statement repayment allocation")
	}
	return nil
}

func projectStatementRepayment(
	repayment *creditRepaymentRow,
	statement *creditStatementRow,
	account *creditAccountRow,
	reused bool,
) *CreditRepayment {
	if repayment == nil || statement == nil || account == nil {
		return nil
	}
	return &CreditRepayment{
		RepaymentNo:     repayment.RepaymentNo,
		CreditAccountNo: repayment.CreditAccountNo,
		CustomerNo:      repayment.CustomerNo, CurrencyCode: repayment.CurrencyCode,
		RepaymentType:          repayment.RepaymentType,
		FundingSourceType:      repayment.FundingSourceType,
		FundingSourceReference: repayment.FundingSourceReference,
		LedgerTransactionNo:    repayment.LedgerTransactionNo,
		AmountMinor:            repayment.AmountMinor,
		AllocatedAmountMinor:   repayment.AllocatedAmountMinor,
		UnallocatedAmountMinor: repayment.UnallocatedAmountMinor,
		StatusCode:             repayment.StatusCode, StatementNo: statement.StatementNo,
		StatementStatusCode:       statement.StatusCode,
		StatementOutstandingMinor: statement.OutstandingMinor,
		AccountStatusCode:         account.StatusCode,
		AccountAvailableMinor:     account.AvailableLimitMinor,
		AccountUnbilledMinor:      account.UnbilledPrincipalMinor,
		AccountBilledMinor:        account.BilledPrincipalMinor,
		AccountDelinquentMinor:    account.DelinquentPrincipalMinor,
		AccountWrittenOffMinor:    account.WrittenOffPrincipalMinor,
		AccountVersion:            account.Version, Reused: reused,
	}
}

func minCreditAmount(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
