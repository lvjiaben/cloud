package billing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumCreditStatementLines = 10_000

type creditTermsRow struct {
	TermsNo                 string `orm:"terms_no"`
	TermsVersion            int64  `orm:"terms_version"`
	BillingDay              int    `orm:"billing_day"`
	RepaymentPeriodDays     int    `orm:"repayment_period_days"`
	GracePeriodDays         int    `orm:"grace_period_days"`
	DailyLateFeeBasisPoints int    `orm:"daily_late_fee_basis_points"`
	MaximumLateFeeMinor     int64  `orm:"maximum_late_fee_minor"`
}

type creditStatementRow struct {
	StatementNo      string    `orm:"statement_no"`
	CreditAccountNo  string    `orm:"credit_account_no"`
	CustomerNo       string    `orm:"customer_no"`
	CurrencyCode     string    `orm:"currency_code"`
	TermsNo          string    `orm:"terms_no"`
	TermsVersion     int64     `orm:"terms_version"`
	PeriodStartedAt  time.Time `orm:"period_started_at"`
	PeriodEndedAt    time.Time `orm:"period_ended_at"`
	LineCount        int       `orm:"line_count"`
	PrincipalMinor   int64     `orm:"principal_minor"`
	FeeMinor         int64     `orm:"fee_minor"`
	TotalAmountMinor int64     `orm:"total_amount_minor"`
	PaidAmountMinor  int64     `orm:"paid_amount_minor"`
	WrittenOffMinor  int64     `orm:"written_off_amount_minor"`
	OutstandingMinor int64     `orm:"outstanding_amount_minor"`
	StatusCode       string    `orm:"status_code"`
	IssuedAt         time.Time `orm:"issued_at"`
	PaymentDueAt     time.Time `orm:"payment_due_at"`
	Version          int64     `orm:"version"`
}

type creditStatementExposure struct {
	ID                       int64  `orm:"id"`
	ExposureNo               string `orm:"exposure_no"`
	UnbilledPrincipalMinor   int64  `orm:"unbilled_principal_minor"`
	BilledPrincipalMinor     int64  `orm:"billed_principal_minor"`
	RepaidPrincipalMinor     int64  `orm:"repaid_principal_minor"`
	ReleasedPrincipalMinor   int64  `orm:"released_principal_minor"`
	WrittenOffPrincipalMinor int64  `orm:"written_off_principal_minor"`
	Version                  int64  `orm:"version"`
}

type creditStatementLinePayload struct {
	LineNumber      int    `json:"line_number"`
	StatementLineNo string `json:"statement_line_no"`
	ExposureNo      string `json:"exposure_no"`
	AmountMinor     int64  `json:"amount_minor"`
	ExposureVersion int64  `json:"exposure_version"`
	ContentDigest   string `json:"content_digest"`
}

func (s *serviceImpl) IssueCreditStatement(
	ctx context.Context,
	input CreditStatementIssueInput,
) (*CreditStatement, error) {
	normalized, err := normalizeCreditStatementIssue(input)
	if err != nil {
		return nil, err
	}
	var result *CreditStatement
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		account, statementNo, replay, loadErr := s.loadCreditStatementRequest(ctx, normalized)
		if loadErr != nil {
			return loadErr
		}
		if replay != nil {
			result = replay
			return nil
		}
		terms, termsErr := s.validateCreditStatementPeriod(ctx, normalized, account)
		if termsErr != nil {
			return termsErr
		}
		payload, principalMinor, payloadErr := s.buildCreditStatementLines(
			ctx, normalized, account, statementNo,
		)
		if payloadErr != nil {
			return payloadErr
		}
		var clock struct {
			IssuedAt     time.Time `orm:"issued_at"`
			PaymentDueAt time.Time `orm:"payment_due_at"`
		}
		if loadErr = s.database.GetScan(ctx, &clock, `
SELECT CURRENT_TIMESTAMP AS issued_at,
       CURRENT_TIMESTAMP + make_interval(days => ?) AS payment_due_at`,
			terms.RepaymentPeriodDays); loadErr != nil {
			return gerror.Wrap(loadErr, "derive credit statement dates")
		}
		statementDigest, digestErr := contentDigest(struct {
			StatementNo, CreditAccountNo, TermsNo, CurrencyCode    string
			TermsVersion                                           int64
			PeriodStartedAt, PeriodEndedAt, IssuedAt, PaymentDueAt time.Time
			PrincipalMinor                                         int64
			Lines                                                  []creditStatementLinePayload
		}{
			statementNo, account.CreditAccountNo, terms.TermsNo,
			account.CurrencyCode, terms.TermsVersion,
			normalized.PeriodStartedAt, normalized.PeriodEndedAt,
			clock.IssuedAt, clock.PaymentDueAt, principalMinor, payload,
		})
		if digestErr != nil {
			return digestErr
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement (
    tenant_id, statement_no, credit_account_no, customer_no, currency_code,
    terms_no, terms_version, period_started_at, period_ended_at, line_count,
    principal_minor, fee_minor, adjustment_minor, total_amount_minor,
    paid_amount_minor, written_off_amount_minor, outstanding_amount_minor,
    status_code, content_digest, issued_at, payment_due_at, version
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, 0, 0, ?,
          'open', ?, ?, ?, 1)`, statementNo, account.CreditAccountNo,
			account.CustomerNo, account.CurrencyCode, terms.TermsNo,
			terms.TermsVersion, normalized.PeriodStartedAt,
			normalized.PeriodEndedAt, len(payload), principalMinor,
			principalMinor, principalMinor, statementDigest,
			clock.IssuedAt, clock.PaymentDueAt); insertErr != nil {
			return gerror.Wrap(insertErr, "create credit statement")
		}
		encodedPayload, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return gerror.Wrap(encodeErr, "encode credit statement lines")
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement_line (
    tenant_id, statement_line_no, statement_no, credit_account_no,
    customer_no, currency_code, line_number, line_type_code, exposure_no,
    source_type_code, source_reference, amount_minor, content_digest
)
SELECT 0, line.statement_line_no, ?, ?, ?, ?, line.line_number,
       'principal', line.exposure_no, 'exposure', line.exposure_no,
       line.amount_minor, line.content_digest
FROM jsonb_to_recordset(?::jsonb) AS line(
    statement_line_no TEXT, exposure_no TEXT, line_number INT,
    amount_minor BIGINT, exposure_version BIGINT, content_digest TEXT
)`, statementNo, account.CreditAccountNo, account.CustomerNo,
			account.CurrencyCode, string(encodedPayload)); insertErr != nil {
			return gerror.Wrap(insertErr, "append credit statement lines")
		}
		updatedExposures, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure exposure
SET billed_principal_minor = exposure.billed_principal_minor + line.amount_minor,
    unbilled_principal_minor = exposure.unbilled_principal_minor - line.amount_minor,
    status_code = CASE WHEN exposure.repaid_principal_minor > 0
                       THEN 'partially_repaid' ELSE 'billed' END,
    version = exposure.version + 1, updated_at = CURRENT_TIMESTAMP
FROM jsonb_to_recordset(?::jsonb) AS line(
    exposure_no TEXT, amount_minor BIGINT, exposure_version BIGINT
)
WHERE exposure.tenant_id = 0 AND exposure.exposure_no = line.exposure_no
  AND exposure.version = line.exposure_version
  AND exposure.unbilled_principal_minor = line.amount_minor`, string(encodedPayload))
		if updateErr != nil {
			return gerror.Wrap(updateErr, "mark credit exposures billed")
		}
		updatedCount, rowsErr := updatedExposures.RowsAffected()
		if rowsErr != nil || updatedCount != int64(len(payload)) {
			return bizerr.NewCode(CodeCreditConflict)
		}
		afterUnbilled := account.UnbilledPrincipalMinor - principalMinor
		afterBilled := account.BilledPrincipalMinor + principalMinor
		updatedAccount, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET unbilled_principal_minor = ?, billed_principal_minor = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
			afterUnbilled, afterBilled, account.CreditAccountNo, account.Version)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "move credit principal into statement")
		}
		updatedCount, rowsErr = updatedAccount.RowsAffected()
		if rowsErr != nil || updatedCount != 1 {
			return bizerr.NewCode(CodeCreditConflict)
		}
		if movementErr := s.appendCreditMovement(ctx, creditMovementInput{
			MovementNo: deterministicCreditNo("crm_", "bill", statementNo),
			Account:    account, StatementNo: statementNo, MovementType: "bill",
			AmountMinor: principalMinor, StatusAfter: account.StatusCode,
			AvailableAfter: account.AvailableLimitMinor,
			UnbilledAfter:  afterUnbilled, BilledAfter: afterBilled,
			DelinquentAfter: account.DelinquentPrincipalMinor,
			WrittenOffAfter: account.WrittenOffPrincipalMinor,
			SourceType:      "statement", SourceReference: statementNo,
			EvidenceDigest: statementDigest, ActorType: "system", ActorID: 0,
			OccurredAt: clock.IssuedAt,
		}); movementErr != nil {
			return movementErr
		}
		account.UnbilledPrincipalMinor = afterUnbilled
		account.BilledPrincipalMinor = afterBilled
		account.Version++
		created, loadErr := loadCreditStatement(ctx, s.database, statementNo)
		if loadErr != nil {
			return loadErr
		}
		if created == nil {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		result = projectCreditStatement(created, account, false)
		return nil
	})
	return creditStatementResult(result, err)
}

func (s *serviceImpl) loadCreditStatementRequest(
	ctx context.Context,
	input CreditStatementIssueInput,
) (*creditAccountRow, string, *CreditStatement, error) {
	account, err := loadCreditAccountByNo(ctx, s.database, input.CreditAccountNo, true)
	if err != nil {
		return nil, "", nil, err
	}
	if account == nil {
		return nil, "", nil, bizerr.NewCode(CodeCreditNotFound)
	}
	if !validCreditAccountProjection(account) ||
		(account.StatusCode != "active" && account.StatusCode != "frozen" &&
			account.StatusCode != "expired") {
		return nil, "", nil, bizerr.NewCode(CodeCreditConflict)
	}
	statementNo := deterministicCreditNo(
		"crs_", "statement", account.CreditAccountNo, input.IdempotencyKey,
	)
	existing, err := loadCreditStatement(ctx, s.database, statementNo)
	if err != nil {
		return nil, "", nil, err
	}
	if existing == nil {
		return account, statementNo, nil, nil
	}
	if existing.CreditAccountNo != account.CreditAccountNo ||
		!existing.PeriodStartedAt.Equal(input.PeriodStartedAt) ||
		!existing.PeriodEndedAt.Equal(input.PeriodEndedAt) {
		return nil, "", nil, bizerr.NewCode(CodeCreditConflict)
	}
	replay := projectCreditStatement(existing, account, true)
	if replay == nil {
		return nil, "", nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	return account, statementNo, replay, nil
}

func (s *serviceImpl) validateCreditStatementPeriod(
	ctx context.Context,
	input CreditStatementIssueInput,
	account *creditAccountRow,
) (*creditTermsRow, error) {
	if input.PeriodEndedAt.After(account.Clock) {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	terms, err := loadCreditTerms(
		ctx, s.database, account.CreditAccountNo,
		account.CurrentTermsNo, account.CurrentTermsVersion,
	)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	timezone, err := loadCreditMerchantTimezone(ctx, s.database)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	if input.PeriodEndedAt.In(location).Day() != terms.BillingDay {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	overlap, err := s.database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_credit_statement
WHERE tenant_id = 0 AND credit_account_no = ?
  AND period_started_at < ? AND period_ended_at > ?`,
		account.CreditAccountNo, input.PeriodEndedAt, input.PeriodStartedAt)
	if err != nil {
		return nil, gerror.Wrap(err, "inspect overlapping credit statements")
	}
	if overlap.Int64() != 0 {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	return terms, nil
}

func (s *serviceImpl) buildCreditStatementLines(
	ctx context.Context,
	input CreditStatementIssueInput,
	account *creditAccountRow,
	statementNo string,
) ([]creditStatementLinePayload, int64, error) {
	exposures := make([]creditStatementExposure, 0, 64)
	if err := s.database.GetScan(ctx, &exposures, `
SELECT id, exposure_no, unbilled_principal_minor, billed_principal_minor,
       repaid_principal_minor, released_principal_minor,
       written_off_principal_minor, version
FROM plugin_idc_core_credit_exposure
WHERE tenant_id = 0 AND credit_account_no = ?
  AND unbilled_principal_minor > 0 AND created_at < ?
ORDER BY created_at ASC, id ASC
LIMIT ? FOR UPDATE`, account.CreditAccountNo, input.PeriodEndedAt,
		maximumCreditStatementLines+1); err != nil {
		return nil, 0, gerror.Wrap(err, "lock unbilled credit exposures")
	}
	if len(exposures) == 0 || len(exposures) > maximumCreditStatementLines {
		return nil, 0, bizerr.NewCode(CodeCreditConflict)
	}
	payload := make([]creditStatementLinePayload, 0, len(exposures))
	principalMinor := int64(0)
	for index, exposure := range exposures {
		if exposure.UnbilledPrincipalMinor <= 0 ||
			principalMinor > maximumAmountMinor-exposure.UnbilledPrincipalMinor {
			return nil, 0, bizerr.NewCode(CodeCreditIntegrity)
		}
		principalMinor += exposure.UnbilledPrincipalMinor
		lineNo := deterministicCreditNo(
			"crl_", "statement_line", statementNo, exposure.ExposureNo,
		)
		lineDigest, err := contentDigest(struct {
			StatementLineNo, StatementNo, ExposureNo string
			LineNumber                               int
			AmountMinor                              int64
		}{lineNo, statementNo, exposure.ExposureNo, index + 1, exposure.UnbilledPrincipalMinor})
		if err != nil {
			return nil, 0, err
		}
		payload = append(payload, creditStatementLinePayload{
			LineNumber: index + 1, StatementLineNo: lineNo,
			ExposureNo:      exposure.ExposureNo,
			AmountMinor:     exposure.UnbilledPrincipalMinor,
			ExposureVersion: exposure.Version, ContentDigest: lineDigest,
		})
	}
	if principalMinor <= 0 || principalMinor > account.UnbilledPrincipalMinor {
		return nil, 0, bizerr.NewCode(CodeCreditIntegrity)
	}
	return payload, principalMinor, nil
}

func loadCreditTerms(
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	termsNo string,
	termsVersion int64,
) (*creditTermsRow, error) {
	var row *creditTermsRow
	if err := database.GetScan(ctx, &row, `
SELECT terms_no, terms_version, billing_day, repayment_period_days,
       grace_period_days, daily_late_fee_basis_points, maximum_late_fee_minor
FROM plugin_idc_core_credit_account_terms
WHERE tenant_id = 0 AND credit_account_no = ?
  AND terms_no = ? AND terms_version = ?`,
		creditAccountNo, termsNo, termsVersion); err != nil {
		return nil, gerror.Wrap(err, "load credit account terms")
	}
	return row, nil
}

func loadCreditMerchantTimezone(ctx context.Context, database gdb.DB) (string, error) {
	rows := make([]struct {
		Timezone string `orm:"timezone"`
	}, 0, 2)
	if err := database.GetScan(ctx, &rows, `
SELECT timezone FROM plugin_idc_core_merchant_profile
WHERE tenant_id = 0 ORDER BY id ASC LIMIT 2`); err != nil {
		return "", gerror.Wrap(err, "load credit merchant timezone")
	}
	if len(rows) != 1 || rows[0].Timezone == "" {
		return "", bizerr.NewCode(CodeCreditIntegrity)
	}
	return rows[0].Timezone, nil
}

func loadCreditStatement(
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
WHERE tenant_id = 0 AND statement_no = ?`, statementNo); err != nil {
		return nil, gerror.Wrap(err, "load credit statement")
	}
	return row, nil
}

func projectCreditStatement(
	statement *creditStatementRow,
	account *creditAccountRow,
	reused bool,
) *CreditStatement {
	if statement == nil || account == nil {
		return nil
	}
	return &CreditStatement{
		StatementNo:     statement.StatementNo,
		CreditAccountNo: statement.CreditAccountNo,
		CustomerNo:      statement.CustomerNo, CurrencyCode: statement.CurrencyCode,
		TermsNo: statement.TermsNo, TermsVersion: statement.TermsVersion,
		PeriodStartedAt: statement.PeriodStartedAt,
		PeriodEndedAt:   statement.PeriodEndedAt, LineCount: statement.LineCount,
		PrincipalMinor:   statement.PrincipalMinor,
		TotalAmountMinor: statement.TotalAmountMinor,
		OutstandingMinor: statement.OutstandingMinor,
		StatusCode:       statement.StatusCode, IssuedAt: statement.IssuedAt,
		PaymentDueAt:          statement.PaymentDueAt,
		AccountAvailableMinor: account.AvailableLimitMinor,
		AccountUnbilledMinor:  account.UnbilledPrincipalMinor,
		AccountBilledMinor:    account.BilledPrincipalMinor,
		AccountVersion:        account.Version, Reused: reused,
	}
}

func creditStatementResult(result *CreditStatement, err error) (*CreditStatement, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}
