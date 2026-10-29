// This file posts authorized credit write-offs and recoveries against matching
// immutable ledger evidence; one operator may act as requester and processor.

package billing

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

var creditWriteoffReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)

type creditWriteoffRow struct {
	WriteoffNo             string    `orm:"writeoff_no"`
	StatementNo            string    `orm:"statement_no"`
	CreditAccountNo        string    `orm:"credit_account_no"`
	CustomerNo             string    `orm:"customer_no"`
	CurrencyCode           string    `orm:"currency_code"`
	AmountMinor            int64     `orm:"amount_minor"`
	LedgerTransactionNo    string    `orm:"ledger_transaction_no"`
	ReasonCode             string    `orm:"reason_code"`
	RequestedBy            int       `orm:"requested_by"`
	ReviewedBy             int       `orm:"reviewed_by"`
	ApprovalEvidenceDigest string    `orm:"approval_evidence_digest"`
	PostedAt               time.Time `orm:"posted_at"`
}

type creditWriteoffExposure struct {
	ExposureNo           string `json:"exposure_no" orm:"exposure_no"`
	BilledPrincipalMinor int64  `json:"billed_principal_minor" orm:"billed_principal_minor"`
	Version              int64  `json:"version" orm:"version"`
}

type creditWriteoffLedgerEvidence struct {
	TransactionCount int64 `orm:"transaction_count"`
	EntryCount       int64 `orm:"entry_count"`
	ExpenseMinor     int64 `orm:"expense_minor"`
	ReceivableMinor  int64 `orm:"receivable_minor"`
	OtherMinor       int64 `orm:"other_minor"`
}

func (s *serviceImpl) WriteOffCreditStatement(
	ctx context.Context,
	input CreditWriteoffInput,
) (*CreditWriteoff, error) {
	normalized, err := normalizeCreditWriteoff(input)
	if err != nil {
		return nil, err
	}
	var result *CreditWriteoff
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var transactionErr error
		result, transactionErr = s.writeOffCreditStatement(ctx, normalized)
		return transactionErr
	})
	return creditWriteoffResult(result, err)
}

func normalizeCreditWriteoff(input CreditWriteoffInput) (CreditWriteoffInput, error) {
	if !exactlyTrimmed(
		input.WriteoffNo, input.StatementNo, input.LedgerTransactionNo,
		input.ReasonCode, input.ApprovalEvidenceDigest,
	) {
		return CreditWriteoffInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.WriteoffNo = strings.TrimSpace(input.WriteoffNo)
	input.StatementNo = strings.TrimSpace(input.StatementNo)
	input.LedgerTransactionNo = strings.TrimSpace(input.LedgerTransactionNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.ApprovalEvidenceDigest = strings.TrimSpace(input.ApprovalEvidenceDigest)
	if !validBusinessReference(input.WriteoffNo, 40) ||
		!validBusinessReference(input.StatementNo, 40) ||
		!validAmount(input.ExpectedOutstandingMinor) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) ||
		!creditWriteoffReasonPattern.MatchString(input.ReasonCode) ||
		input.RequestedBy <= 0 || input.ReviewedBy <= 0 ||
		!canonicalDigestPattern.MatchString(input.ApprovalEvidenceDigest) {
		return CreditWriteoffInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func (s *serviceImpl) writeOffCreditStatement(
	ctx context.Context,
	input CreditWriteoffInput,
) (*CreditWriteoff, error) {
	existing, err := loadCreditWriteoff(ctx, s.database, input.WriteoffNo)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.projectCreditWriteoffReplay(ctx, input, existing)
	}
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
	statement, err := loadCreditStatementForUpdate(ctx, s.database, input.StatementNo)
	if err != nil {
		return nil, err
	}
	caseRow, err := loadCreditDelinquencyByStatement(ctx, s.database, input.StatementNo, true)
	if err != nil {
		return nil, err
	}
	if !validCreditWriteoffTarget(input, statement, account, caseRow) {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	exposures, principalMinor, err := loadCreditWriteoffExposures(
		ctx, s.database, statement.StatementNo,
	)
	if err != nil {
		return nil, err
	}
	if principalMinor <= 0 || principalMinor != caseRow.PrincipalMinor ||
		principalMinor > account.BilledPrincipalMinor ||
		principalMinor > account.DelinquentPrincipalMinor {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	if err = validateCreditWriteoffLedger(ctx, s.database, account,
		input, statement.OutstandingMinor); err != nil {
		return nil, err
	}
	contentDigest, err := contentDigest(struct {
		CreditWriteoffInput
		CreditAccountNo, CustomerNo, CurrencyCode string
		PrincipalMinor                            int64
		Exposures                                 []creditWriteoffExposure
	}{input, account.CreditAccountNo, account.CustomerNo, account.CurrencyCode,
		principalMinor, exposures})
	if err != nil {
		return nil, err
	}
	if err = s.persistCreditWriteoff(
		ctx, input, statement, account, caseRow, exposures,
		principalMinor, contentDigest,
	); err != nil {
		return nil, err
	}
	created, err := loadCreditWriteoff(ctx, s.database, input.WriteoffNo)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return projectCreditWriteoff(created, statement, account, false), nil
}

func validCreditWriteoffTarget(
	input CreditWriteoffInput,
	statement *creditStatementRow,
	account *creditAccountRow,
	caseRow *creditDelinquencyRow,
) bool {
	if statement == nil || caseRow == nil || statement.CreditAccountNo != account.CreditAccountNo ||
		caseRow.CreditAccountNo != account.CreditAccountNo ||
		caseRow.StatementNo != statement.StatementNo || statement.StatusCode != "overdue" ||
		statement.WrittenOffMinor != 0 || statement.OutstandingMinor != input.ExpectedOutstandingMinor ||
		caseRow.OutstandingAmountMinor != statement.OutstandingMinor ||
		account.StatusCode != "frozen" {
		return false
	}
	return caseRow.StatusCode == "frozen" || caseRow.StatusCode == "service_action_pending"
}

func loadCreditWriteoffExposures(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
) ([]creditWriteoffExposure, int64, error) {
	rows := make([]creditWriteoffExposure, 0, 64)
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
ORDER BY line.line_number, line.id
FOR UPDATE OF exposure`, statementNo); err != nil {
		return nil, 0, gerror.Wrap(err, "lock credit write-off exposures")
	}
	if len(rows) == 0 || len(rows) > maximumCreditStatementLines {
		return nil, 0, bizerr.NewCode(CodeCreditIntegrity)
	}
	var total int64
	for _, row := range rows {
		if !validAmount(row.BilledPrincipalMinor) ||
			total > maximumAmountMinor-row.BilledPrincipalMinor {
			return nil, 0, bizerr.NewCode(CodeCreditIntegrity)
		}
		total += row.BilledPrincipalMinor
	}
	return rows, total, nil
}

func validateCreditWriteoffLedger(
	ctx context.Context,
	database gdb.DB,
	account *creditAccountRow,
	input CreditWriteoffInput,
	amountMinor int64,
) error {
	transaction, err := lockInvoiceLedgerTransaction(ctx, database, input.LedgerTransactionNo)
	if err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return bizerr.NewCode(CodeCreditConflict)
		}
		return err
	}
	if transaction.TransactionType != "writeoff" || transaction.OriginalTransaction != "" ||
		transaction.CurrencyCode != account.CurrencyCode ||
		transaction.AmountMinor != amountMinor {
		return bizerr.NewCode(CodeCreditConflict)
	}
	var evidence creditWriteoffLedgerEvidence
	if err = database.GetScan(ctx, &evidence, `
SELECT
    COUNT(DISTINCT transaction.transaction_no) AS transaction_count,
    COUNT(entry.id) AS entry_count,
    COALESCE(SUM(entry.amount_minor) FILTER (
        WHERE entry.direction_code = 'debit'
          AND entry.entry_role_code = 'expense'
          AND ledger_account.owner_type_code = 'merchant'
          AND ledger_account.account_type_code = 'bad_debt_expense'
          AND merchant.enabled
    ), 0) AS expense_minor,
    COALESCE(SUM(entry.amount_minor) FILTER (
        WHERE entry.direction_code = 'credit'
          AND entry.entry_role_code = 'receivable'
          AND ledger_account.owner_type_code = 'merchant'
          AND ledger_account.account_type_code = 'accounts_receivable'
          AND merchant.enabled
    ), 0) AS receivable_minor,
    COALESCE(SUM(entry.amount_minor) FILTER (
        WHERE NOT ((entry.direction_code = 'debit'
                    AND entry.entry_role_code = 'expense'
                    AND ledger_account.owner_type_code = 'merchant'
                    AND ledger_account.account_type_code = 'bad_debt_expense'
                    AND merchant.enabled)
               OR (entry.direction_code = 'credit'
                    AND entry.entry_role_code = 'receivable'
                    AND ledger_account.owner_type_code = 'merchant'
                    AND ledger_account.account_type_code = 'accounts_receivable'
                    AND merchant.enabled))
    ), 0) AS other_minor
FROM plugin_idc_core_billing_ledger_transaction transaction
JOIN plugin_idc_core_billing_ledger_entry entry
  ON entry.transaction_no = transaction.transaction_no
JOIN plugin_idc_core_billing_ledger_account ledger_account
  ON ledger_account.account_no = entry.account_no
LEFT JOIN plugin_idc_core_merchant_profile merchant
  ON merchant.tenant_id = 0
 AND merchant.merchant_no = ledger_account.owner_reference
WHERE transaction.tenant_id = 0 AND transaction.transaction_no = ?
  AND transaction.source_type_code = 'credit_writeoff'
  AND transaction.source_reference = ?`, input.LedgerTransactionNo,
		input.WriteoffNo); err != nil {
		return gerror.Wrap(err, "validate credit write-off ledger")
	}
	if evidence.TransactionCount != 1 || evidence.EntryCount != 2 ||
		evidence.ExpenseMinor != amountMinor || evidence.ReceivableMinor != amountMinor ||
		evidence.OtherMinor != 0 {
		return bizerr.NewCode(CodeCreditConflict)
	}
	if err = requireUnreversedLedgerTransaction(ctx, database, input.LedgerTransactionNo); err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return bizerr.NewCode(CodeCreditConflict)
		}
		return err
	}
	return nil
}

func (s *serviceImpl) persistCreditWriteoff(
	ctx context.Context,
	input CreditWriteoffInput,
	statement *creditStatementRow,
	account *creditAccountRow,
	caseRow *creditDelinquencyRow,
	exposures []creditWriteoffExposure,
	principalMinor int64,
	contentDigest string,
) error {
	if _, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_writeoff (
    tenant_id, writeoff_no, statement_no, credit_account_no, customer_no,
    currency_code, writeoff_type_code, amount_minor, ledger_transaction_no,
    reason_code, requested_by, reviewed_by, approval_evidence_digest,
    content_digest, posted_at
) VALUES (0, ?, ?, ?, ?, ?, 'writeoff', ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.WriteoffNo, statement.StatementNo, account.CreditAccountNo,
		account.CustomerNo, account.CurrencyCode, statement.OutstandingMinor,
		input.LedgerTransactionNo, input.ReasonCode, input.RequestedBy,
		input.ReviewedBy, input.ApprovalEvidenceDigest, contentDigest,
		account.Clock); err != nil {
		return gerror.Wrap(err, "create credit write-off evidence")
	}
	updatedStatement, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET written_off_amount_minor = written_off_amount_minor + outstanding_amount_minor,
    outstanding_amount_minor = 0, status_code = 'written_off',
    written_off_at = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND statement_no = ? AND version = ?`,
		account.Clock, statement.StatementNo, statement.Version)
	if err != nil {
		return gerror.Wrap(err, "write off credit statement")
	}
	if err = requireSingleCreditUpdate(updatedStatement); err != nil {
		return err
	}
	encoded, err := json.Marshal(exposures)
	if err != nil {
		return gerror.Wrap(err, "encode credit write-off exposures")
	}
	updatedExposures, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure exposure
SET billed_principal_minor = exposure.billed_principal_minor - item.billed_principal_minor,
    written_off_principal_minor = exposure.written_off_principal_minor
        + item.billed_principal_minor,
    status_code = 'written_off', version = exposure.version + 1,
    updated_at = CURRENT_TIMESTAMP
FROM jsonb_to_recordset(?::jsonb) AS item(
    exposure_no TEXT, billed_principal_minor BIGINT, version BIGINT
)
WHERE exposure.tenant_id = 0 AND exposure.exposure_no = item.exposure_no
  AND exposure.version = item.version
  AND exposure.billed_principal_minor = item.billed_principal_minor`, string(encoded))
	if err != nil {
		return gerror.Wrap(err, "write off credit exposures")
	}
	updatedCount, err := updatedExposures.RowsAffected()
	if err != nil || updatedCount != int64(len(exposures)) {
		return bizerr.NewCode(CodeCreditConflict)
	}
	afterBilled := account.BilledPrincipalMinor - principalMinor
	afterDelinquent := account.DelinquentPrincipalMinor - principalMinor
	afterWrittenOff := account.WrittenOffPrincipalMinor + principalMinor
	updatedAccount, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET billed_principal_minor = ?, delinquent_principal_minor = ?,
    written_off_principal_minor = ?, status_code = 'bad_debt',
    freeze_reason_code = 'credit.bad_debt', version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		afterBilled, afterDelinquent, afterWrittenOff,
		account.CreditAccountNo, account.Version)
	if err != nil {
		return gerror.Wrap(err, "mark credit account bad debt")
	}
	if err = requireSingleCreditUpdate(updatedAccount); err != nil {
		return err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo("crm_", "writeoff", input.WriteoffNo),
		Account:    account, StatementNo: statement.StatementNo,
		MovementType: "writeoff", AmountMinor: principalMinor,
		StatusAfter: "bad_debt", AvailableAfter: account.AvailableLimitMinor,
		UnbilledAfter: account.UnbilledPrincipalMinor, BilledAfter: afterBilled,
		DelinquentAfter: afterDelinquent, WrittenOffAfter: afterWrittenOff,
		SourceType: "writeoff", SourceReference: input.WriteoffNo,
		EvidenceDigest: input.ApprovalEvidenceDigest,
		ActorType:      "staff", ActorID: input.ReviewedBy, OccurredAt: account.Clock,
	}); err != nil {
		return err
	}
	updatedCase, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_delinquency_case
SET status_code = 'written_off', written_off_at = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND version = ?`,
		account.Clock, caseRow.CaseNo, caseRow.Version)
	if err != nil {
		return gerror.Wrap(err, "mark credit delinquency written off")
	}
	if err = requireSingleCreditUpdate(updatedCase); err != nil {
		return err
	}
	statement.WrittenOffMinor += statement.OutstandingMinor
	statement.OutstandingMinor = 0
	statement.StatusCode = "written_off"
	statement.Version++
	account.BilledPrincipalMinor = afterBilled
	account.DelinquentPrincipalMinor = afterDelinquent
	account.WrittenOffPrincipalMinor = afterWrittenOff
	account.StatusCode = "bad_debt"
	account.Version++
	return nil
}

func loadCreditWriteoff(
	ctx context.Context,
	database gdb.DB,
	writeoffNo string,
) (*creditWriteoffRow, error) {
	var row *creditWriteoffRow
	if err := database.GetScan(ctx, &row, `
SELECT writeoff_no, statement_no, credit_account_no, customer_no, currency_code,
       amount_minor, ledger_transaction_no, reason_code, requested_by,
       reviewed_by, approval_evidence_digest, posted_at
FROM plugin_idc_core_credit_writeoff
WHERE tenant_id = 0 AND writeoff_no = ?`, writeoffNo); err != nil {
		return nil, gerror.Wrap(err, "load credit write-off")
	}
	return row, nil
}

func (s *serviceImpl) projectCreditWriteoffReplay(
	ctx context.Context,
	input CreditWriteoffInput,
	row *creditWriteoffRow,
) (*CreditWriteoff, error) {
	if row.StatementNo != input.StatementNo || row.AmountMinor != input.ExpectedOutstandingMinor ||
		row.LedgerTransactionNo != input.LedgerTransactionNo ||
		row.ReasonCode != input.ReasonCode || row.RequestedBy != input.RequestedBy ||
		row.ReviewedBy != input.ReviewedBy ||
		row.ApprovalEvidenceDigest != input.ApprovalEvidenceDigest {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	account, err := loadCreditAccountByNo(ctx, s.database, row.CreditAccountNo, false)
	if err != nil {
		return nil, err
	}
	statement, err := loadCreditStatement(ctx, s.database, row.StatementNo)
	if err != nil {
		return nil, err
	}
	if account == nil || statement == nil || !validCreditAccountProjection(account) {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return projectCreditWriteoff(row, statement, account, true), nil
}

func projectCreditWriteoff(
	row *creditWriteoffRow,
	statement *creditStatementRow,
	account *creditAccountRow,
	reused bool,
) *CreditWriteoff {
	if row == nil || statement == nil || account == nil {
		return nil
	}
	return &CreditWriteoff{
		WriteoffNo: row.WriteoffNo, StatementNo: row.StatementNo,
		CreditAccountNo: row.CreditAccountNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor,
		LedgerTransactionNo: row.LedgerTransactionNo, ReasonCode: row.ReasonCode,
		RequestedBy: row.RequestedBy, ReviewedBy: row.ReviewedBy, PostedAt: row.PostedAt,
		StatementStatusCode:       statement.StatusCode,
		StatementOutstandingMinor: statement.OutstandingMinor,
		AccountStatusCode:         account.StatusCode,
		AccountAvailableMinor:     account.AvailableLimitMinor,
		AccountBilledMinor:        account.BilledPrincipalMinor,
		AccountDelinquentMinor:    account.DelinquentPrincipalMinor,
		AccountWrittenOffMinor:    account.WrittenOffPrincipalMinor,
		AccountVersion:            account.Version, Reused: reused,
	}
}

func creditWriteoffResult(result *CreditWriteoff, err error) (*CreditWriteoff, error) {
	if err == nil {
		if result == nil {
			return nil, bizerr.NewCode(CodeCreditIntegrity)
		}
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}
