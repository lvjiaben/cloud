package billing

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditRecoveryExposure struct {
	ExposureNo      string `json:"exposure_no" orm:"exposure_no"`
	AmountMinor     int64  `json:"amount_minor" orm:"amount_minor"`
	ExposureVersion int64  `json:"exposure_version" orm:"version"`
}

func (s *serviceImpl) ApplyConfirmedBadDebtRecovery(
	ctx context.Context,
	input CreditStatementRepaymentInput,
) (*CreditRepayment, error) {
	normalized, err := normalizeCreditStatementRepayment(input)
	if err != nil {
		return nil, err
	}
	if normalized.FundingSourceType == "migration" {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	var result *CreditRepayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var transactionErr error
		result, transactionErr = s.applyConfirmedBadDebtRecovery(ctx, normalized)
		return transactionErr
	})
	return creditRepaymentResult(result, err)
}

func (s *serviceImpl) applyConfirmedBadDebtRecovery(
	ctx context.Context,
	input CreditStatementRepaymentInput,
) (*CreditRepayment, error) {
	identity, err := loadCreditStatement(ctx, s.database, input.StatementNo)
	if err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	account, err := loadCreditAccountByNo(ctx, s.database, identity.CreditAccountNo, true)
	if err != nil {
		return nil, err
	}
	if account == nil || !validCreditAccountProjection(account) {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	existing, err := loadCreditRepaymentBySource(
		ctx, s.database, input.FundingSourceType, input.FundingSourceReference,
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return projectBadDebtRecoveryReplay(input, existing, identity, account)
	}
	statement, err := loadCreditStatementForUpdate(ctx, s.database, input.StatementNo)
	if err != nil {
		return nil, err
	}
	caseRow, err := loadCreditDelinquencyByStatement(ctx, s.database, input.StatementNo, true)
	if err != nil {
		return nil, err
	}
	if !validBadDebtRecoveryTarget(input, statement, account, caseRow) {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	if err = validateBadDebtRecoveryHistory(
		ctx, s.database, statement, caseRow,
	); err != nil {
		return nil, err
	}
	if err = validateBadDebtRecoveryLedger(ctx, s.database, account, input); err != nil {
		return nil, err
	}
	feePayment := minCreditAmount(input.AmountMinor, caseRow.LateFeeMinor)
	principalPayment := input.AmountMinor - feePayment
	exposures, err := loadCreditRecoveryExposures(
		ctx, s.database, statement.StatementNo, principalPayment,
	)
	if err != nil {
		return nil, err
	}
	repaymentNo := deterministicCreditNo(
		"crr_", "bad-debt-recovery", input.FundingSourceType,
		input.FundingSourceReference,
	)
	if err = s.persistBadDebtRecovery(
		ctx, input, repaymentNo, statement, account, caseRow,
		exposures, feePayment, principalPayment,
	); err != nil {
		return nil, err
	}
	created, err := loadCreditRepaymentBySource(
		ctx, s.database, input.FundingSourceType, input.FundingSourceReference,
	)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return projectStatementRepayment(created, statement, account, false), nil
}

func validateBadDebtRecoveryHistory(
	ctx context.Context,
	database gdb.DB,
	statement *creditStatementRow,
	caseRow *creditDelinquencyRow,
) error {
	var facts struct {
		WriteoffCount  int64 `orm:"writeoff_count"`
		WriteoffMinor  int64 `orm:"writeoff_minor"`
		RecoveredMinor int64 `orm:"recovered_minor"`
	}
	if err := database.GetScan(ctx, &facts, `
SELECT
    (SELECT COUNT(*)
     FROM plugin_idc_core_credit_writeoff
     WHERE tenant_id = 0 AND statement_no = ?
       AND writeoff_type_code = 'writeoff') AS writeoff_count,
    (SELECT COALESCE(SUM(amount_minor), 0)
     FROM plugin_idc_core_credit_writeoff
     WHERE tenant_id = 0 AND statement_no = ?
       AND writeoff_type_code = 'writeoff') AS writeoff_minor,
    (SELECT COALESCE(SUM(CASE allocation.direction_code
        WHEN 'apply' THEN allocation.amount_minor
        ELSE -allocation.amount_minor END), 0)
     FROM plugin_idc_core_credit_repayment_allocation allocation
     WHERE allocation.tenant_id = 0 AND allocation.statement_no = ?
       AND allocation.allocation_type_code = 'written_off_recovery') AS recovered_minor`,
		statement.StatementNo, statement.StatementNo, statement.StatementNo); err != nil {
		return gerror.Wrap(err, "validate bad debt recovery history")
	}
	if facts.WriteoffCount != 1 || facts.WriteoffMinor != statement.WrittenOffMinor ||
		facts.RecoveredMinor < 0 || facts.RecoveredMinor > statement.WrittenOffMinor ||
		statement.WrittenOffMinor-facts.RecoveredMinor != caseRow.OutstandingAmountMinor {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	return nil
}

func validBadDebtRecoveryTarget(
	input CreditStatementRepaymentInput,
	statement *creditStatementRow,
	account *creditAccountRow,
	caseRow *creditDelinquencyRow,
) bool {
	return statement != nil && caseRow != nil &&
		statement.CreditAccountNo == account.CreditAccountNo &&
		caseRow.CreditAccountNo == account.CreditAccountNo &&
		caseRow.StatementNo == statement.StatementNo &&
		statement.StatusCode == "written_off" && statement.OutstandingMinor == 0 &&
		caseRow.StatusCode == "written_off" && account.StatusCode == "bad_debt" &&
		input.AmountMinor <= caseRow.OutstandingAmountMinor &&
		caseRow.PrincipalMinor <= account.WrittenOffPrincipalMinor
}

func projectBadDebtRecoveryReplay(
	input CreditStatementRepaymentInput,
	existing *creditRepaymentRow,
	statement *creditStatementRow,
	account *creditAccountRow,
) (*CreditRepayment, error) {
	if existing.RepaymentType != "bad_debt_recovery" ||
		existing.StatementNo != input.StatementNo ||
		existing.LedgerTransactionNo != input.LedgerTransactionNo ||
		existing.AmountMinor != input.AmountMinor ||
		existing.SourceSnapshotDigest != input.SourceSnapshotDigest ||
		existing.ConfirmationEvidenceDigest != input.ConfirmationEvidenceDigest {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	result := projectStatementRepayment(existing, statement, account, true)
	if result == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return result, nil
}

func validateBadDebtRecoveryLedger(
	ctx context.Context,
	database gdb.DB,
	account *creditAccountRow,
	input CreditStatementRepaymentInput,
) error {
	var transaction *transactionRow
	if err := database.GetScan(ctx, &transaction, `
SELECT id, transaction_no, transaction_type_code, currency_code,
       source_type_code, source_reference, source_snapshot_digest,
       COALESCE(original_transaction_no, '') AS original_transaction_no,
       entry_count, debit_total_minor, credit_total_minor, content_digest
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND transaction_no = ? AND currency_code = ?
FOR UPDATE`, input.LedgerTransactionNo, account.CurrencyCode); err != nil {
		return gerror.Wrap(err, "lock bad debt recovery ledger transaction")
	}
	if transaction == nil || transaction.SourceType != "credit_repayment" ||
		transaction.SourceReference != input.FundingSourceReference ||
		transaction.TransactionType != expectedCreditRepaymentTransactionType(
			input.FundingSourceType,
		) || transaction.OriginalTransactionNo != "" || transaction.EntryCount != 2 ||
		transaction.DebitTotalMinor != input.AmountMinor ||
		transaction.CreditTotalMinor != input.AmountMinor {
		return bizerr.NewCode(CodeCreditConflict)
	}
	var evidence struct {
		ExpenseCredits  int64 `orm:"expense_credits"`
		CashDebits      int64 `orm:"cash_debits"`
		BalanceDebits   int64 `orm:"balance_debits"`
		UnexpectedCount int64 `orm:"unexpected_count"`
	}
	if err := database.GetScan(ctx, &evidence, `
SELECT
    COUNT(*) FILTER (
        WHERE entry.direction_code = 'credit'
          AND entry.entry_role_code = 'expense'
          AND ledger_account.owner_type_code = 'merchant'
          AND ledger_account.account_type_code = 'bad_debt_expense'
          AND merchant.enabled
          AND entry.amount_minor = ?
    ) AS expense_credits,
    COUNT(*) FILTER (
        WHERE entry.direction_code = 'debit'
          AND ledger_account.account_type_code = 'merchant_cash'
          AND merchant.enabled
          AND entry.amount_minor = ?
    ) AS cash_debits,
    COUNT(*) FILTER (
        WHERE entry.direction_code = 'debit'
          AND ledger_account.account_type_code = 'customer_balance'
          AND ledger_account.customer_no = ?
          AND entry.amount_minor = ?
    ) AS balance_debits,
    COUNT(*) FILTER (
        WHERE ledger_account.account_type_code IN (
            'customer_commission', 'payment_clearing', 'accounts_receivable'
        )
    ) AS unexpected_count
FROM plugin_idc_core_billing_ledger_entry entry
JOIN plugin_idc_core_billing_ledger_account ledger_account
  ON ledger_account.account_no = entry.account_no
LEFT JOIN plugin_idc_core_merchant_profile merchant
  ON merchant.tenant_id = 0
 AND merchant.merchant_no = ledger_account.owner_reference
WHERE entry.tenant_id = 0 AND entry.transaction_no = ?`,
		input.AmountMinor, input.AmountMinor, account.CustomerNo,
		input.AmountMinor, input.LedgerTransactionNo); err != nil {
		return gerror.Wrap(err, "validate bad debt recovery ledger entries")
	}
	requiresCash := input.FundingSourceType == "online_payment" ||
		input.FundingSourceType == "manual_receipt"
	if evidence.ExpenseCredits != 1 || evidence.UnexpectedCount != 0 ||
		(requiresCash && evidence.CashDebits != 1) ||
		(input.FundingSourceType == "customer_balance" && evidence.BalanceDebits != 1) {
		return bizerr.NewCode(CodeCreditConflict)
	}
	if err := requireUnreversedLedgerTransaction(
		ctx, database, input.LedgerTransactionNo,
	); err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return bizerr.NewCode(CodeCreditConflict)
		}
		return err
	}
	return nil
}

func loadCreditRecoveryExposures(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
	amountMinor int64,
) ([]creditRecoveryExposure, error) {
	if amountMinor == 0 {
		return nil, nil
	}
	rows := make([]struct {
		ExposureNo      string `orm:"exposure_no"`
		WrittenOffMinor int64  `orm:"written_off_principal_minor"`
		Version         int64  `orm:"version"`
	}, 0, 64)
	if err := database.GetScan(ctx, &rows, `
SELECT exposure.exposure_no, exposure.written_off_principal_minor,
       exposure.version
FROM plugin_idc_core_credit_statement_line line
JOIN plugin_idc_core_credit_exposure exposure
  ON exposure.exposure_no = line.exposure_no
 AND exposure.credit_account_no = line.credit_account_no
 AND exposure.customer_no = line.customer_no
 AND exposure.currency_code = line.currency_code
WHERE line.tenant_id = 0 AND line.statement_no = ?
  AND line.line_type_code = 'principal'
  AND exposure.written_off_principal_minor > 0
ORDER BY line.line_number, line.id
FOR UPDATE OF exposure`, statementNo); err != nil {
		return nil, gerror.Wrap(err, "lock bad debt recovery exposures")
	}
	if len(rows) > maximumCreditStatementLines {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	remaining := amountMinor
	payload := make([]creditRecoveryExposure, 0, len(rows))
	for _, row := range rows {
		if remaining == 0 {
			break
		}
		allocated := minCreditAmount(remaining, row.WrittenOffMinor)
		payload = append(payload, creditRecoveryExposure{
			ExposureNo: row.ExposureNo, AmountMinor: allocated,
			ExposureVersion: row.Version,
		})
		remaining -= allocated
	}
	if remaining != 0 {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return payload, nil
}

func (s *serviceImpl) persistBadDebtRecovery(
	ctx context.Context,
	input CreditStatementRepaymentInput,
	repaymentNo string,
	statement *creditStatementRow,
	account *creditAccountRow,
	caseRow *creditDelinquencyRow,
	exposures []creditRecoveryExposure,
	feePayment int64,
	principalPayment int64,
) error {
	if _, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment (
    tenant_id, repayment_no, credit_account_no, customer_no, currency_code,
    repayment_type_code, funding_source_type_code, funding_source_reference,
    ledger_transaction_no, amount_minor, allocated_amount_minor,
    unallocated_amount_minor, refunded_amount_minor, status_code,
    source_snapshot_digest, confirmation_evidence_digest, confirmed_at, version
) VALUES (0, ?, ?, ?, ?, 'bad_debt_recovery', ?, ?, ?, ?, ?, 0, 0,
          'allocated', ?, ?, ?, 1)`, repaymentNo, account.CreditAccountNo,
		account.CustomerNo, account.CurrencyCode, input.FundingSourceType,
		input.FundingSourceReference, input.LedgerTransactionNo, input.AmountMinor,
		input.AmountMinor, input.SourceSnapshotDigest,
		input.ConfirmationEvidenceDigest, account.Clock); err != nil {
		return gerror.Wrap(err, "create confirmed bad debt recovery")
	}
	allocationNo := deterministicCreditNo("crl_", "bad-debt-recovery", repaymentNo)
	allocationDigest, err := contentDigest(struct {
		AllocationNo, RepaymentNo, CreditAccountNo, StatementNo string
		AmountMinor                                             int64
	}{allocationNo, repaymentNo, account.CreditAccountNo,
		statement.StatementNo, input.AmountMinor})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment_allocation (
    tenant_id, allocation_no, repayment_no, credit_account_no, customer_no,
    currency_code, statement_no, allocation_type_code, direction_code,
    amount_minor, source_reference, content_digest
) VALUES (0, ?, ?, ?, ?, ?, ?, 'written_off_recovery', 'apply', ?, ?, ?)`,
		allocationNo, repaymentNo, account.CreditAccountNo, account.CustomerNo,
		account.CurrencyCode, statement.StatementNo, input.AmountMinor,
		allocationNo, allocationDigest); err != nil {
		return gerror.Wrap(err, "append bad debt recovery allocation")
	}
	if err = s.updateBadDebtRecoveryExposures(ctx, exposures); err != nil {
		return err
	}
	afterAvailable := account.AvailableLimitMinor + principalPayment
	afterWrittenOff := account.WrittenOffPrincipalMinor - principalPayment
	updatedAccount, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = ?, written_off_principal_minor = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		afterAvailable, afterWrittenOff, account.CreditAccountNo, account.Version)
	if err != nil {
		return gerror.Wrap(err, "recover written-off credit limit")
	}
	if err = requireSingleCreditUpdate(updatedAccount); err != nil {
		return err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo("crm_", "recovery", repaymentNo),
		Account:    account, StatementNo: statement.StatementNo, RepaymentNo: repaymentNo,
		MovementType: "recovery", AmountMinor: principalPayment,
		StatusAfter: account.StatusCode, AvailableAfter: afterAvailable,
		UnbilledAfter:   account.UnbilledPrincipalMinor,
		BilledAfter:     account.BilledPrincipalMinor,
		DelinquentAfter: account.DelinquentPrincipalMinor,
		WrittenOffAfter: afterWrittenOff, SourceType: "repayment",
		SourceReference: repaymentNo, EvidenceDigest: input.ConfirmationEvidenceDigest,
		ActorType: "system", ActorID: 0, OccurredAt: account.Clock,
	}); err != nil {
		return err
	}
	afterPrincipal := caseRow.PrincipalMinor - principalPayment
	afterFee := caseRow.LateFeeMinor - feePayment
	afterOutstanding := caseRow.OutstandingAmountMinor - input.AmountMinor
	afterStatus := "written_off"
	var resolvedAt any
	if afterOutstanding == 0 {
		afterStatus = "resolved"
		resolvedAt = account.Clock
	}
	updatedCase, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_delinquency_case
SET principal_minor = ?, late_fee_minor = ?, outstanding_amount_minor = ?,
    status_code = ?, resolved_at = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND version = ?`, afterPrincipal,
		afterFee, afterOutstanding, afterStatus, resolvedAt,
		caseRow.CaseNo, caseRow.Version)
	if err != nil {
		return gerror.Wrap(err, "apply recovery to written-off delinquency")
	}
	if err = requireSingleCreditUpdate(updatedCase); err != nil {
		return err
	}
	account.AvailableLimitMinor = afterAvailable
	account.WrittenOffPrincipalMinor = afterWrittenOff
	account.Version++
	return nil
}

func (s *serviceImpl) updateBadDebtRecoveryExposures(
	ctx context.Context,
	exposures []creditRecoveryExposure,
) error {
	if len(exposures) == 0 {
		return nil
	}
	encoded, err := json.Marshal(exposures)
	if err != nil {
		return gerror.Wrap(err, "encode bad debt recovery exposures")
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure exposure
SET written_off_principal_minor = exposure.written_off_principal_minor
        - item.amount_minor,
    repaid_principal_minor = exposure.repaid_principal_minor + item.amount_minor,
    status_code = CASE
        WHEN exposure.written_off_principal_minor - item.amount_minor = 0
            THEN 'settled'
        ELSE 'written_off'
    END,
    version = exposure.version + 1, updated_at = CURRENT_TIMESTAMP
FROM jsonb_to_recordset(?::jsonb) AS item(
    exposure_no TEXT, amount_minor BIGINT, exposure_version BIGINT
)
WHERE exposure.tenant_id = 0 AND exposure.exposure_no = item.exposure_no
  AND exposure.version = item.exposure_version
  AND exposure.written_off_principal_minor >= item.amount_minor`, string(encoded))
	if err != nil {
		return gerror.Wrap(err, "recover written-off credit exposures")
	}
	count, err := updated.RowsAffected()
	if err != nil || count != int64(len(exposures)) {
		return bizerr.NewCode(CodeCreditConflict)
	}
	return nil
}
