package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

const creditServiceActionTaskType = "credit.delinquency.service_action"

type creditDelinquencyServiceImpl struct {
	*serviceImpl
	tasks asynctask.Service
}

type creditDelinquencyRow struct {
	CaseNo                   string     `orm:"case_no"`
	StatementNo              string     `orm:"statement_no"`
	CreditAccountNo          string     `orm:"credit_account_no"`
	CustomerNo               string     `orm:"customer_no"`
	CurrencyCode             string     `orm:"currency_code"`
	TermsNo                  string     `orm:"terms_no"`
	TermsVersion             int64      `orm:"terms_version"`
	StatusCode               string     `orm:"status_code"`
	PrincipalMinor           int64      `orm:"principal_minor"`
	LateFeeMinor             int64      `orm:"late_fee_minor"`
	OutstandingAmountMinor   int64      `orm:"outstanding_amount_minor"`
	OpenedAt                 time.Time  `orm:"opened_at"`
	GraceUntil               time.Time  `orm:"grace_until"`
	FrozenAt                 *time.Time `orm:"frozen_at"`
	ServiceActionRequestedAt *time.Time `orm:"service_action_requested_at"`
	ResolvedAt               *time.Time `orm:"resolved_at"`
	ReasonCode               string     `orm:"reason_code"`
	SourceSnapshotDigest     string     `orm:"source_snapshot_digest"`
	Version                  int64      `orm:"version"`
}

type creditOutstandingAllocation struct {
	PrincipalPaidMinor int64 `orm:"principal_paid_minor"`
	FeePaidMinor       int64 `orm:"fee_paid_minor"`
}

func (s *creditDelinquencyServiceImpl) OpenDueCreditStatement(
	ctx context.Context,
	statementNo string,
) (*CreditDelinquency, error) {
	if !exactlyTrimmed(statementNo) || !validBusinessReference(statementNo, 40) {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	var result *CreditDelinquency
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var transactionErr error
		result, transactionErr = s.openDueCreditStatement(ctx, statementNo)
		return transactionErr
	})
	return creditDelinquencyResult(result, err)
}

func (s *creditDelinquencyServiceImpl) openDueCreditStatement(
	ctx context.Context,
	statementNo string,
) (*CreditDelinquency, error) {
	identity, err := loadCreditStatement(ctx, s.database, statementNo)
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
	statement, err := loadCreditStatementForUpdate(ctx, s.database, statementNo)
	if err != nil {
		return nil, err
	}
	existing, err := loadCreditDelinquencyByStatement(ctx, s.database, statementNo, true)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return projectCreditDelinquency(existing, account, "", true), nil
	}
	if statement == nil || statement.CreditAccountNo != account.CreditAccountNo ||
		(statement.StatusCode != "open" && statement.StatusCode != "partially_paid") ||
		statement.OutstandingMinor <= 0 || !account.Clock.After(statement.PaymentDueAt) ||
		(account.StatusCode != "active" && account.StatusCode != "expired" &&
			account.StatusCode != "frozen") {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	terms, err := loadCreditTerms(
		ctx, s.database, account.CreditAccountNo, statement.TermsNo, statement.TermsVersion,
	)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	allocation, err := loadCreditOutstandingAllocation(ctx, s.database, statement.StatementNo)
	if err != nil {
		return nil, err
	}
	principalMinor := statement.PrincipalMinor - allocation.PrincipalPaidMinor
	lateFeeMinor := statement.FeeMinor - allocation.FeePaidMinor
	if principalMinor <= 0 || lateFeeMinor < 0 ||
		principalMinor+lateFeeMinor != statement.OutstandingMinor ||
		account.DelinquentPrincipalMinor > account.BilledPrincipalMinor-principalMinor {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	clock := deriveCreditDelinquencyClock(
		statement.PaymentDueAt, account.Clock, terms.GracePeriodDays,
	)
	caseNo := deterministicCreditNo("crc_", "delinquency", statement.StatementNo)
	evidenceDigest, err := contentDigest(struct {
		CaseNo, StatementNo, CreditAccountNo, CustomerNo, CurrencyCode string
		TermsNo, ReasonCode                                            string
		TermsVersion, PrincipalMinor, LateFeeMinor                     int64
		OpenedAt, GraceUntil, DetectedAt                               time.Time
	}{
		caseNo, statement.StatementNo, account.CreditAccountNo,
		account.CustomerNo, account.CurrencyCode, terms.TermsNo,
		"credit.overdue", terms.TermsVersion, principalMinor,
		lateFeeMinor, clock.OpenedAt, clock.GraceUntil, clock.DetectedAt,
	})
	if err != nil {
		return nil, err
	}
	if err = s.persistOpenedCreditDelinquency(
		ctx, statement, account, terms, caseNo, evidenceDigest,
		principalMinor, lateFeeMinor, clock,
	); err != nil {
		return nil, err
	}
	created, err := loadCreditDelinquencyByStatement(ctx, s.database, statementNo, false)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return projectCreditDelinquency(created, account, "", false), nil
}

type creditDelinquencyClock struct {
	OpenedAt   time.Time `orm:"opened_at"`
	GraceUntil time.Time `orm:"grace_until"`
	DetectedAt time.Time `orm:"detected_at"`
}

func deriveCreditDelinquencyClock(
	paymentDueAt time.Time,
	detectedAt time.Time,
	gracePeriodDays int,
) creditDelinquencyClock {
	return creditDelinquencyClock{
		OpenedAt: paymentDueAt, GraceUntil: paymentDueAt.AddDate(0, 0, gracePeriodDays),
		DetectedAt: detectedAt,
	}
}

func loadCreditOutstandingAllocation(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
) (creditOutstandingAllocation, error) {
	var allocation creditOutstandingAllocation
	if err := database.GetScan(ctx, &allocation, `
SELECT
    COALESCE(SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
        FILTER (WHERE allocation_type_code = 'statement_principal'), 0) AS principal_paid_minor,
    COALESCE(SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
        FILTER (WHERE allocation_type_code = 'statement_fee'), 0) AS fee_paid_minor
FROM plugin_idc_core_credit_repayment_allocation
WHERE tenant_id = 0 AND statement_no = ?`, statementNo); err != nil {
		return creditOutstandingAllocation{}, gerror.Wrap(err, "load credit delinquency allocations")
	}
	return allocation, nil
}

func (s *creditDelinquencyServiceImpl) persistOpenedCreditDelinquency(
	ctx context.Context,
	statement *creditStatementRow,
	account *creditAccountRow,
	terms *creditTermsRow,
	caseNo string,
	evidenceDigest string,
	principalMinor int64,
	lateFeeMinor int64,
	clock creditDelinquencyClock,
) error {
	updatedStatement, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET status_code = 'overdue', overdue_at = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND statement_no = ? AND version = ?`,
		clock.DetectedAt, statement.StatementNo, statement.Version)
	if err != nil {
		return gerror.Wrap(err, "mark credit statement overdue")
	}
	if err = requireSingleCreditUpdate(updatedStatement); err != nil {
		return err
	}
	afterDelinquent := account.DelinquentPrincipalMinor + principalMinor
	updatedAccount, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET delinquent_principal_minor = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		afterDelinquent, account.CreditAccountNo, account.Version)
	if err != nil {
		return gerror.Wrap(err, "mark credit principal delinquent")
	}
	if err = requireSingleCreditUpdate(updatedAccount); err != nil {
		return err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo("crm_", "delinquency", caseNo),
		Account:    account, StatementNo: statement.StatementNo,
		MovementType: "delinquency", AmountMinor: principalMinor,
		StatusAfter: account.StatusCode, AvailableAfter: account.AvailableLimitMinor,
		UnbilledAfter: account.UnbilledPrincipalMinor,
		BilledAfter:   account.BilledPrincipalMinor, DelinquentAfter: afterDelinquent,
		WrittenOffAfter: account.WrittenOffPrincipalMinor,
		SourceType:      "delinquency", SourceReference: caseNo,
		EvidenceDigest: evidenceDigest, ActorType: "system", ActorID: 0,
		OccurredAt: clock.DetectedAt,
	}); err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_delinquency_case (
    tenant_id, case_no, statement_no, credit_account_no, customer_no,
    currency_code, terms_no, terms_version, status_code, principal_minor,
    late_fee_minor, outstanding_amount_minor, opened_at, grace_until,
    reason_code, source_snapshot_digest, version
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, 'grace_period', ?, ?, ?, ?, ?,
          'credit.overdue', ?, 1)`,
		caseNo, statement.StatementNo, account.CreditAccountNo, account.CustomerNo,
		account.CurrencyCode, terms.TermsNo, terms.TermsVersion, principalMinor,
		lateFeeMinor, principalMinor+lateFeeMinor, clock.OpenedAt, clock.GraceUntil,
		evidenceDigest); err != nil {
		return gerror.Wrap(err, "create credit delinquency case")
	}
	statement.StatusCode = "overdue"
	statement.Version++
	account.DelinquentPrincipalMinor = afterDelinquent
	account.Version++
	return nil
}

func (s *creditDelinquencyServiceImpl) EscalateCreditDelinquency(
	ctx context.Context,
	caseNo string,
) (*CreditDelinquency, error) {
	if !exactlyTrimmed(caseNo) || !validBusinessReference(caseNo, 40) {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	var result *CreditDelinquency
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var transactionErr error
		result, transactionErr = s.escalateCreditDelinquency(ctx, caseNo)
		return transactionErr
	})
	return creditDelinquencyResult(result, err)
}

func (s *creditDelinquencyServiceImpl) escalateCreditDelinquency(
	ctx context.Context,
	caseNo string,
) (*CreditDelinquency, error) {
	identity, err := loadCreditDelinquencyByCase(ctx, s.database, caseNo, false)
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
	statement, err := loadCreditStatementForUpdate(ctx, s.database, identity.StatementNo)
	if err != nil {
		return nil, err
	}
	caseRow, err := loadCreditDelinquencyByCase(ctx, s.database, caseNo, true)
	if err != nil {
		return nil, err
	}
	if caseRow == nil || statement == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	if caseRow.StatusCode == "written_off" || caseRow.StatusCode == "resolved" {
		return projectCreditDelinquency(caseRow, account, "", true), nil
	}
	if statement.StatusCode != "overdue" ||
		statement.OutstandingMinor != caseRow.OutstandingAmountMinor {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	if caseRow.StatusCode != "grace_period" && caseRow.StatusCode != "frozen" &&
		caseRow.StatusCode != "service_action_pending" {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	if caseRow.StatusCode == "grace_period" && account.Clock.Before(caseRow.GraceUntil) {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	if caseRow.StatusCode != "service_action_pending" {
		if err = s.freezeCreditAccountForDelinquency(ctx, caseRow, account); err != nil {
			return nil, err
		}
		if err = s.markCreditServiceActionPending(ctx, caseRow, account.Clock); err != nil {
			return nil, err
		}
	}
	task, err := s.createCreditServiceActionTask(ctx, caseRow)
	if err != nil {
		return nil, err
	}
	return projectCreditDelinquency(caseRow, account, task.Task.TaskNo,
		caseRow.StatusCode == "service_action_pending" && task.Reused), nil
}

func (s *creditDelinquencyServiceImpl) freezeCreditAccountForDelinquency(
	ctx context.Context,
	caseRow *creditDelinquencyRow,
	account *creditAccountRow,
) error {
	if account.StatusCode == "frozen" {
		return nil
	}
	if account.StatusCode != "active" && account.StatusCode != "expired" {
		return bizerr.NewCode(CodeCreditConflict)
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET status_code = 'frozen', freeze_reason_code = 'credit.delinquency',
    frozen_at = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		account.Clock, account.CreditAccountNo, account.Version)
	if err != nil {
		return gerror.Wrap(err, "freeze delinquent credit account")
	}
	if err = requireSingleCreditUpdate(updated); err != nil {
		return err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo("crm_", "freeze", caseRow.CaseNo),
		Account:    account, StatementNo: caseRow.StatementNo,
		MovementType: "freeze", AmountMinor: 0, StatusAfter: "frozen",
		AvailableAfter:  account.AvailableLimitMinor,
		UnbilledAfter:   account.UnbilledPrincipalMinor,
		BilledAfter:     account.BilledPrincipalMinor,
		DelinquentAfter: account.DelinquentPrincipalMinor,
		WrittenOffAfter: account.WrittenOffPrincipalMinor,
		SourceType:      "delinquency", SourceReference: caseRow.CaseNo + ":freeze",
		EvidenceDigest: caseRow.SourceSnapshotDigest,
		ActorType:      "system", ActorID: 0, OccurredAt: account.Clock,
	}); err != nil {
		return err
	}
	account.StatusCode = "frozen"
	account.Version++
	return nil
}

func (s *creditDelinquencyServiceImpl) markCreditServiceActionPending(
	ctx context.Context,
	caseRow *creditDelinquencyRow,
	clock time.Time,
) error {
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_delinquency_case
SET status_code = 'service_action_pending', frozen_at = COALESCE(frozen_at, ?),
    service_action_requested_at = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND version = ?`,
		clock, clock, caseRow.CaseNo, caseRow.Version)
	if err != nil {
		return gerror.Wrap(err, "request delinquency service action")
	}
	if err = requireSingleCreditUpdate(updated); err != nil {
		return err
	}
	caseRow.StatusCode = "service_action_pending"
	if caseRow.FrozenAt == nil {
		frozenAt := clock
		caseRow.FrozenAt = &frozenAt
	}
	requestedAt := clock
	caseRow.ServiceActionRequestedAt = &requestedAt
	caseRow.Version++
	return nil
}

func (s *creditDelinquencyServiceImpl) createCreditServiceActionTask(
	ctx context.Context,
	caseRow *creditDelinquencyRow,
) (*asynctask.CreateResult, error) {
	payload, err := json.Marshal(struct {
		CaseNo, StatementNo, CreditAccountNo, CustomerNo, CurrencyCode string
	}{
		caseRow.CaseNo, caseRow.StatementNo, caseRow.CreditAccountNo,
		caseRow.CustomerNo, caseRow.CurrencyCode,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "encode credit service action task")
	}
	result, err := s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
		TaskType: creditServiceActionTaskType, SubjectType: "credit_delinquency_case",
		SubjectNo: caseRow.CaseNo, CustomerNo: caseRow.CustomerNo,
		IdempotencyKey: caseRow.CaseNo + ":service-action:v1",
		Priority:       100, Payload: payload,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "create credit service action task")
	}
	return result, nil
}

func loadCreditDelinquencyByStatement(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
	lock bool,
) (*creditDelinquencyRow, error) {
	return loadCreditDelinquency(ctx, database, "statement_no = ?", statementNo, lock)
}

func loadCreditDelinquencyByCase(
	ctx context.Context,
	database gdb.DB,
	caseNo string,
	lock bool,
) (*creditDelinquencyRow, error) {
	return loadCreditDelinquency(ctx, database, "case_no = ?", caseNo, lock)
}

func loadCreditDelinquency(
	ctx context.Context,
	database gdb.DB,
	predicate string,
	argument string,
	lock bool,
) (*creditDelinquencyRow, error) {
	locking := ""
	if lock {
		locking = " FOR UPDATE"
	}
	var row *creditDelinquencyRow
	if err := database.GetScan(ctx, &row, `
SELECT case_no, statement_no, credit_account_no, customer_no, currency_code,
       terms_no, terms_version, status_code, principal_minor, late_fee_minor,
       outstanding_amount_minor, opened_at, grace_until, frozen_at,
       service_action_requested_at, resolved_at, reason_code,
       source_snapshot_digest, version
FROM plugin_idc_core_credit_delinquency_case
WHERE tenant_id = 0 AND `+predicate+locking, argument); err != nil {
		return nil, gerror.Wrap(err, "load credit delinquency case")
	}
	return row, nil
}

func projectCreditDelinquency(
	row *creditDelinquencyRow,
	account *creditAccountRow,
	taskNo string,
	reused bool,
) *CreditDelinquency {
	if row == nil || account == nil {
		return nil
	}
	return &CreditDelinquency{
		CaseNo: row.CaseNo, StatementNo: row.StatementNo,
		CreditAccountNo: row.CreditAccountNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, StatusCode: row.StatusCode,
		PrincipalMinor: row.PrincipalMinor, LateFeeMinor: row.LateFeeMinor,
		OutstandingAmountMinor: row.OutstandingAmountMinor,
		OpenedAt:               row.OpenedAt, GraceUntil: row.GraceUntil,
		FrozenAt: row.FrozenAt, ServiceActionRequestedAt: row.ServiceActionRequestedAt,
		ResolvedAt: row.ResolvedAt, AccountStatusCode: account.StatusCode,
		AccountDelinquentMinor: account.DelinquentPrincipalMinor,
		AccountVersion:         account.Version, ServiceActionTaskNo: taskNo, Reused: reused,
	}
}

func requireSingleCreditUpdate(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return bizerr.NewCode(CodeCreditConflict)
	}
	return nil
}

func creditDelinquencyResult(
	result *CreditDelinquency,
	err error,
) (*CreditDelinquency, error) {
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
