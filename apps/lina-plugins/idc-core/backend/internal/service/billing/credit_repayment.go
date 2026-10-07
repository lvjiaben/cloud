package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditRepaymentRow struct {
	RepaymentNo                string `orm:"repayment_no"`
	CreditAccountNo            string `orm:"credit_account_no"`
	CustomerNo                 string `orm:"customer_no"`
	CurrencyCode               string `orm:"currency_code"`
	RepaymentType              string `orm:"repayment_type_code"`
	FundingSourceType          string `orm:"funding_source_type_code"`
	FundingSourceReference     string `orm:"funding_source_reference"`
	LedgerTransactionNo        string `orm:"ledger_transaction_no"`
	AmountMinor                int64  `orm:"amount_minor"`
	AllocatedAmountMinor       int64  `orm:"allocated_amount_minor"`
	UnallocatedAmountMinor     int64  `orm:"unallocated_amount_minor"`
	StatusCode                 string `orm:"status_code"`
	SourceSnapshotDigest       string `orm:"source_snapshot_digest"`
	ConfirmationEvidenceDigest string `orm:"confirmation_evidence_digest"`
	ExposureNo                 string `orm:"exposure_no"`
	StatementNo                string `orm:"statement_no"`
}

func (s *serviceImpl) ApplyConfirmedCreditPrepayment(
	ctx context.Context,
	input CreditPrepaymentInput,
) (*CreditRepayment, error) {
	normalized, err := normalizeCreditPrepayment(input)
	if err != nil {
		return nil, err
	}
	var result *CreditRepayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var transactionErr error
		result, transactionErr = s.applyConfirmedCreditPrepayment(ctx, normalized)
		return transactionErr
	})
	return creditRepaymentResult(result, err)
}

func (s *serviceImpl) applyConfirmedCreditPrepayment(
	ctx context.Context,
	input CreditPrepaymentInput,
) (*CreditRepayment, error) {
	identity, err := loadCreditExposureByNo(ctx, s.database, input.ExposureNo, false)
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
		return s.replayConfirmedCreditPrepayment(ctx, input, existing, account)
	}
	exposure, err := loadCreditExposureByNo(ctx, s.database, input.ExposureNo, true)
	if err != nil {
		return nil, err
	}
	if !validCreditPrepaymentTarget(input, exposure, account) {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	if err = validateCreditRepaymentLedger(
		ctx, s.database, account, input.FundingSourceType,
		input.FundingSourceReference, input.LedgerTransactionNo, input.AmountMinor,
	); err != nil {
		return nil, err
	}
	return s.createConfirmedCreditPrepayment(ctx, input, exposure, account)
}

func (s *serviceImpl) replayConfirmedCreditPrepayment(
	ctx context.Context,
	input CreditPrepaymentInput,
	existing *creditRepaymentRow,
	account *creditAccountRow,
) (*CreditRepayment, error) {
	if existing.CreditAccountNo != account.CreditAccountNo ||
		existing.RepaymentType != "prepayment" || existing.ExposureNo != input.ExposureNo ||
		existing.LedgerTransactionNo != input.LedgerTransactionNo ||
		existing.AmountMinor != input.AmountMinor ||
		existing.SourceSnapshotDigest != input.SourceSnapshotDigest ||
		existing.ConfirmationEvidenceDigest != input.ConfirmationEvidenceDigest {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	exposure, err := loadCreditExposureByNo(ctx, s.database, input.ExposureNo, false)
	if err != nil {
		return nil, err
	}
	result := projectCreditRepayment(existing, exposure, account, true)
	if result == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return result, nil
}

func validCreditPrepaymentTarget(
	input CreditPrepaymentInput,
	exposure *creditExposureRow,
	account *creditAccountRow,
) bool {
	return exposure != nil && exposure.CreditAccountNo == account.CreditAccountNo &&
		exposure.UnbilledPrincipalMinor >= input.AmountMinor &&
		(exposure.StatusCode == "unbilled" || exposure.StatusCode == "partially_repaid") &&
		account.UnbilledPrincipalMinor >= input.AmountMinor &&
		account.StatusCode != "closed" && account.StatusCode != "bad_debt"
}

func (s *serviceImpl) createConfirmedCreditPrepayment(
	ctx context.Context,
	input CreditPrepaymentInput,
	exposure *creditExposureRow,
	account *creditAccountRow,
) (*CreditRepayment, error) {
	repaymentNo := deterministicCreditNo(
		"crr_", "prepayment", input.FundingSourceType, input.FundingSourceReference,
	)
	allocationNo := deterministicCreditNo("cal_", "prepayment", repaymentNo)
	allocationDigest, digestErr := contentDigest(struct {
		AllocationNo, RepaymentNo, CreditAccountNo, ExposureNo string
		AmountMinor                                            int64
	}{
		allocationNo, repaymentNo, account.CreditAccountNo,
		exposure.ExposureNo, input.AmountMinor,
	})
	if digestErr != nil {
		return nil, digestErr
	}
	if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment (
    tenant_id, repayment_no, credit_account_no, customer_no, currency_code,
    repayment_type_code, funding_source_type_code, funding_source_reference,
    ledger_transaction_no, amount_minor, allocated_amount_minor,
    unallocated_amount_minor, refunded_amount_minor, status_code,
    source_snapshot_digest, confirmation_evidence_digest, confirmed_at, version
) VALUES (0, ?, ?, ?, ?, 'prepayment', ?, ?, ?, ?, ?, 0, 0,
          'allocated', ?, ?, ?, 1)`, repaymentNo, account.CreditAccountNo,
		account.CustomerNo, account.CurrencyCode, input.FundingSourceType,
		input.FundingSourceReference, input.LedgerTransactionNo,
		input.AmountMinor, input.AmountMinor,
		input.SourceSnapshotDigest, input.ConfirmationEvidenceDigest,
		account.Clock); insertErr != nil {
		return nil, gerror.Wrap(insertErr, "create confirmed credit prepayment")
	}
	if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment_allocation (
    tenant_id, allocation_no, repayment_no, credit_account_no, customer_no,
    currency_code, exposure_no, allocation_type_code, direction_code,
    amount_minor, source_reference, content_digest
) VALUES (0, ?, ?, ?, ?, ?, ?, 'unbilled_principal', 'apply', ?, ?, ?)`,
		allocationNo, repaymentNo, account.CreditAccountNo, account.CustomerNo,
		account.CurrencyCode, exposure.ExposureNo, input.AmountMinor,
		allocationNo, allocationDigest); insertErr != nil {
		return nil, gerror.Wrap(insertErr, "append credit prepayment allocation")
	}
	afterExposureUnbilled := exposure.UnbilledPrincipalMinor - input.AmountMinor
	afterExposureRepaid := exposure.RepaidPrincipalMinor + input.AmountMinor
	afterExposureStatus := "partially_repaid"
	if afterExposureUnbilled == 0 && exposure.BilledPrincipalMinor == 0 {
		afterExposureStatus = "settled"
	}
	updatedExposure, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor = ?, repaid_principal_minor = ?,
    status_code = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND exposure_no = ? AND version = ?`,
		afterExposureUnbilled, afterExposureRepaid, afterExposureStatus,
		exposure.ExposureNo, exposure.Version)
	if updateErr != nil {
		return nil, gerror.Wrap(updateErr, "apply credit prepayment to exposure")
	}
	updatedCount, rowsErr := updatedExposure.RowsAffected()
	if rowsErr != nil || updatedCount != 1 {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	afterAvailable := account.AvailableLimitMinor + input.AmountMinor
	afterUnbilled := account.UnbilledPrincipalMinor - input.AmountMinor
	updatedAccount, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = ?, unbilled_principal_minor = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		afterAvailable, afterUnbilled, account.CreditAccountNo, account.Version)
	if updateErr != nil {
		return nil, gerror.Wrap(updateErr, "restore prepaid credit limit")
	}
	updatedCount, rowsErr = updatedAccount.RowsAffected()
	if rowsErr != nil || updatedCount != 1 {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	if movementErr := s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo("crm_", "prepayment", repaymentNo),
		Account:    account, ExposureNo: exposure.ExposureNo,
		RepaymentNo: repaymentNo, MovementType: "prepayment",
		AmountMinor: input.AmountMinor, StatusAfter: account.StatusCode,
		AvailableAfter: afterAvailable, UnbilledAfter: afterUnbilled,
		BilledAfter:     account.BilledPrincipalMinor,
		DelinquentAfter: account.DelinquentPrincipalMinor,
		WrittenOffAfter: account.WrittenOffPrincipalMinor,
		SourceType:      "repayment", SourceReference: repaymentNo,
		EvidenceDigest: input.ConfirmationEvidenceDigest,
		ActorType:      "system", ActorID: 0, OccurredAt: account.Clock,
	}); movementErr != nil {
		return nil, movementErr
	}
	account.AvailableLimitMinor = afterAvailable
	account.UnbilledPrincipalMinor = afterUnbilled
	account.Version++
	exposure.UnbilledPrincipalMinor = afterExposureUnbilled
	exposure.RepaidPrincipalMinor = afterExposureRepaid
	exposure.StatusCode = afterExposureStatus
	exposure.Version++
	created, loadErr := loadCreditRepaymentBySource(
		ctx, s.database, input.FundingSourceType, input.FundingSourceReference,
	)
	if loadErr != nil {
		return nil, loadErr
	}
	if created == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return projectCreditRepayment(created, exposure, account, false), nil
}

func validateCreditRepaymentLedger(
	ctx context.Context,
	database gdb.DB,
	account *creditAccountRow,
	fundingSourceType string,
	fundingSourceReference string,
	transactionNo string,
	amountMinor int64,
) error {
	var transaction *transactionRow
	if err := database.GetScan(ctx, &transaction, `
SELECT id, transaction_no, transaction_type_code, currency_code,
       source_type_code, source_reference, source_snapshot_digest,
       COALESCE(original_transaction_no, '') AS original_transaction_no,
       entry_count, debit_total_minor, credit_total_minor, content_digest
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND transaction_no = ? AND currency_code = ?`,
		transactionNo, account.CurrencyCode); err != nil {
		return gerror.Wrap(err, "load credit repayment ledger transaction")
	}
	if transaction == nil || transaction.SourceReference != fundingSourceReference ||
		transaction.DebitTotalMinor != amountMinor || transaction.CreditTotalMinor != amountMinor ||
		transaction.EntryCount != 2 || transaction.TransactionType != expectedCreditRepaymentTransactionType(fundingSourceType) {
		return bizerr.NewCode(CodeCreditConflict)
	}
	var evidence struct {
		ReceivableCredits  int64 `orm:"receivable_credits"`
		BalanceDebits      int64 `orm:"balance_debits"`
		MerchantCashDebits int64 `orm:"merchant_cash_debits"`
		CommissionEntries  int64 `orm:"commission_entries"`
	}
	if err := database.GetScan(ctx, &evidence, `
SELECT
    COUNT(*) FILTER (
        WHERE entry.direction_code = 'credit'
          AND ledger_account.account_type_code = 'accounts_receivable'
          AND entry.amount_minor = ?
    ) AS receivable_credits,
    COUNT(*) FILTER (
        WHERE entry.direction_code = 'debit'
          AND ledger_account.account_type_code = 'customer_balance'
          AND ledger_account.customer_no = ?
          AND entry.amount_minor = ?
    ) AS balance_debits,
    COUNT(*) FILTER (
        WHERE entry.direction_code = 'debit'
          AND ledger_account.account_type_code = 'merchant_cash'
          AND entry.amount_minor = ?
    ) AS merchant_cash_debits,
    COUNT(*) FILTER (
        WHERE ledger_account.account_type_code = 'customer_commission'
    ) AS commission_entries
FROM plugin_idc_core_billing_ledger_entry entry
JOIN plugin_idc_core_billing_ledger_account ledger_account
  ON ledger_account.account_no = entry.account_no
WHERE entry.tenant_id = 0 AND entry.transaction_no = ?`,
		amountMinor, account.CustomerNo, amountMinor, amountMinor, transactionNo); err != nil {
		return gerror.Wrap(err, "validate credit repayment ledger entries")
	}
	requiresCashDebit := fundingSourceType == "online_payment" || fundingSourceType == "manual_receipt"
	if evidence.ReceivableCredits != 1 || evidence.CommissionEntries != 0 ||
		(fundingSourceType == "customer_balance" && evidence.BalanceDebits != 1) ||
		(requiresCashDebit && evidence.MerchantCashDebits != 1) {
		return bizerr.NewCode(CodeCreditConflict)
	}
	return nil
}

func expectedCreditRepaymentTransactionType(fundingSourceType string) string {
	switch fundingSourceType {
	case "online_payment":
		return "credit"
	case "customer_balance":
		return "balance_payment"
	case "manual_receipt":
		return "manual_receipt"
	case "migration":
		return "migration"
	default:
		return ""
	}
}

func loadCreditRepaymentBySource(
	ctx context.Context,
	database gdb.DB,
	fundingSourceType string,
	fundingSourceReference string,
) (*creditRepaymentRow, error) {
	var row *creditRepaymentRow
	if err := database.GetScan(ctx, &row, `
SELECT repayment.repayment_no, repayment.credit_account_no,
       repayment.customer_no, repayment.currency_code,
       repayment.repayment_type_code, repayment.funding_source_type_code,
       repayment.funding_source_reference,
       COALESCE(repayment.ledger_transaction_no, '') AS ledger_transaction_no,
       repayment.amount_minor, repayment.allocated_amount_minor,
       repayment.unallocated_amount_minor, repayment.status_code,
       repayment.source_snapshot_digest,
       COALESCE(repayment.confirmation_evidence_digest, '') AS confirmation_evidence_digest,
       COALESCE((SELECT allocation.exposure_no
                 FROM plugin_idc_core_credit_repayment_allocation allocation
                 WHERE allocation.repayment_no = repayment.repayment_no
                   AND allocation.direction_code = 'apply'
                 ORDER BY allocation.id ASC LIMIT 1), '') AS exposure_no,
       COALESCE((SELECT allocation.statement_no
                 FROM plugin_idc_core_credit_repayment_allocation allocation
                 WHERE allocation.repayment_no = repayment.repayment_no
                   AND allocation.direction_code = 'apply'
                 ORDER BY allocation.id ASC LIMIT 1), '') AS statement_no
FROM plugin_idc_core_credit_repayment repayment
WHERE repayment.tenant_id = 0
  AND repayment.funding_source_type_code = ?
  AND repayment.funding_source_reference = ?`,
		fundingSourceType, fundingSourceReference); err != nil {
		return nil, gerror.Wrap(err, "load credit repayment by source")
	}
	return row, nil
}

func projectCreditRepayment(
	repayment *creditRepaymentRow,
	exposure *creditExposureRow,
	account *creditAccountRow,
	reused bool,
) *CreditRepayment {
	if repayment == nil || exposure == nil || account == nil {
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
		StatusCode:             repayment.StatusCode, ExposureNo: exposure.ExposureNo,
		ExposureStatusCode:     exposure.StatusCode,
		AccountStatusCode:      account.StatusCode,
		AccountAvailableMinor:  account.AvailableLimitMinor,
		AccountUnbilledMinor:   account.UnbilledPrincipalMinor,
		AccountBilledMinor:     account.BilledPrincipalMinor,
		AccountDelinquentMinor: account.DelinquentPrincipalMinor,
		AccountVersion:         account.Version, Reused: reused,
	}
}

func creditRepaymentResult(result *CreditRepayment, err error) (*CreditRepayment, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}
