// This file loads and validates bounded customer and management credit-account projections.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditAccountDirectoryRow struct {
	ID                       int64      `orm:"id"`
	CreditAccountNo          string     `orm:"credit_account_no"`
	CustomerNo               string     `orm:"customer_no"`
	CurrencyCode             string     `orm:"currency_code"`
	StatusCode               string     `orm:"status_code"`
	TotalLimitMinor          int64      `orm:"total_limit_minor"`
	AvailableLimitMinor      int64      `orm:"available_limit_minor"`
	UnbilledPrincipalMinor   int64      `orm:"unbilled_principal_minor"`
	BilledPrincipalMinor     int64      `orm:"billed_principal_minor"`
	DelinquentPrincipalMinor int64      `orm:"delinquent_principal_minor"`
	WrittenOffPrincipalMinor int64      `orm:"written_off_principal_minor"`
	ValidFrom                time.Time  `orm:"valid_from"`
	ValidUntil               time.Time  `orm:"valid_until"`
	FreezeReasonCode         *string    `orm:"freeze_reason_code"`
	FrozenAt                 *time.Time `orm:"frozen_at"`
	Version                  int64      `orm:"version"`
	UpdatedAt                time.Time  `orm:"updated_at"`
	TermsNo                  string     `orm:"terms_no"`
	TermsVersion             int64      `orm:"terms_version"`
	TermsTotalLimitMinor     int64      `orm:"terms_total_limit_minor"`
	BillingDay               int        `orm:"billing_day"`
	RepaymentPeriodDays      int        `orm:"repayment_period_days"`
	GracePeriodDays          int        `orm:"grace_period_days"`
	DailyLateFeeBasisPoints  int        `orm:"daily_late_fee_basis_points"`
	MaximumLateFeeMinor      int64      `orm:"maximum_late_fee_minor"`
	EffectiveFrom            time.Time  `orm:"effective_from"`
	EffectiveUntil           time.Time  `orm:"effective_until"`
	CaseNo                   *string    `orm:"case_no"`
	CaseStatementNo          *string    `orm:"case_statement_no"`
	CaseStatusCode           *string    `orm:"case_status_code"`
	CasePrincipalMinor       *int64     `orm:"case_principal_minor"`
	CaseLateFeeMinor         *int64     `orm:"case_late_fee_minor"`
	CaseOutstandingMinor     *int64     `orm:"case_outstanding_minor"`
	CaseOpenedAt             *time.Time `orm:"case_opened_at"`
	CaseGraceUntil           *time.Time `orm:"case_grace_until"`
	CaseResolvedAt           *time.Time `orm:"case_resolved_at"`
}

const creditAccountDirectorySelect = `
SELECT account.id, account.credit_account_no, account.customer_no,
       account.currency_code, account.status_code, account.total_limit_minor,
       account.available_limit_minor, account.unbilled_principal_minor,
       account.billed_principal_minor, account.delinquent_principal_minor,
       account.written_off_principal_minor, account.valid_from, account.valid_until,
       account.freeze_reason_code, account.frozen_at, account.version, account.updated_at,
       terms.terms_no, terms.terms_version,
       terms.total_limit_minor AS terms_total_limit_minor,
       terms.billing_day, terms.repayment_period_days, terms.grace_period_days,
       terms.daily_late_fee_basis_points, terms.maximum_late_fee_minor,
       terms.effective_from, terms.effective_until,
       delinquency.case_no, delinquency.statement_no AS case_statement_no,
       delinquency.status_code AS case_status_code,
       delinquency.principal_minor AS case_principal_minor,
       delinquency.late_fee_minor AS case_late_fee_minor,
       delinquency.outstanding_amount_minor AS case_outstanding_minor,
       delinquency.opened_at AS case_opened_at,
       delinquency.grace_until AS case_grace_until,
       delinquency.resolved_at AS case_resolved_at
FROM plugin_idc_core_credit_account account
JOIN plugin_idc_core_credit_account_terms terms
  ON terms.tenant_id = 0 AND terms.terms_no = account.current_terms_no
 AND terms.credit_account_no = account.credit_account_no
 AND terms.customer_no = account.customer_no AND terms.currency_code = account.currency_code
LEFT JOIN LATERAL (
    SELECT case_no, statement_no, status_code, principal_minor, late_fee_minor,
           outstanding_amount_minor, opened_at, grace_until, resolved_at
    FROM plugin_idc_core_credit_delinquency_case
    WHERE tenant_id = 0 AND credit_account_no = account.credit_account_no
    ORDER BY opened_at DESC, id DESC LIMIT 1
) delinquency ON TRUE
WHERE account.tenant_id = 0`

func (s *accountCreditDirectoryImpl) GetAccountCreditOverview(
	ctx context.Context,
) (*AccountCreditOverview, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	result := &AccountCreditOverview{Accounts: []CreditAccountDirectoryItem{}}
	err = s.store.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		rows := make([]creditAccountDirectoryRow, 0, maximumCreditOverviewAccountCount)
		query := creditAccountDirectorySelect + `
 AND account.customer_no = ?
ORDER BY account.currency_code ASC, account.id ASC LIMIT ?`
		if loadErr := tx.GetScan(&rows, query, audience.CustomerNo, maximumCreditOverviewAccountCount+1); loadErr != nil {
			return gerror.Wrap(loadErr, "load customer credit accounts")
		}
		if len(rows) > maximumCreditOverviewAccountCount {
			return bizerr.NewCode(CodeDirectoryIntegrity)
		}
		for _, row := range rows {
			item, projectErr := projectCreditAccountDirectoryItem(row, false)
			if projectErr != nil {
				return projectErr
			}
			result.Accounts = append(result.Accounts, item)
		}
		var counts *struct {
			PendingApplicationCount int `orm:"pending_application_count"`
			OpenStatementCount      int `orm:"open_statement_count"`
			OverdueStatementCount   int `orm:"overdue_statement_count"`
		}
		if loadErr := tx.GetScan(&counts, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_credit_application
   WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'pending_review')::INT
    AS pending_application_count,
  (SELECT COUNT(*) FROM plugin_idc_core_credit_statement
   WHERE tenant_id = 0 AND customer_no = ?
     AND status_code IN ('open', 'partially_paid', 'overdue'))::INT
    AS open_statement_count,
  (SELECT COUNT(*) FROM plugin_idc_core_credit_statement
   WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'overdue')::INT
    AS overdue_statement_count`, audience.CustomerNo, audience.CustomerNo, audience.CustomerNo); loadErr != nil {
			return gerror.Wrap(loadErr, "load customer credit overview counts")
		}
		if counts == nil || counts.PendingApplicationCount < 0 ||
			counts.OpenStatementCount < 0 || counts.OverdueStatementCount < 0 {
			return bizerr.NewCode(CodeDirectoryIntegrity)
		}
		result.PendingApplicationCount = counts.PendingApplicationCount
		result.OpenStatementCount = counts.OpenStatementCount
		result.OverdueStatementCount = counts.OverdueStatementCount
		return nil
	})
	if err != nil {
		return nil, creditDirectoryResultError(err)
	}
	return result, nil
}

func (s *managementCreditDirectoryImpl) ListManagementCreditAccounts(
	ctx context.Context,
	filter CreditAccountDirectoryFilter,
) (*CreditAccountDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeAccountList(ctx); err != nil {
		return nil, err
	}
	var err error
	filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Cursor, filter.Limit, err =
		normalizeCreditDirectoryFilter(
			filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Cursor, filter.Limit,
		)
	if err != nil {
		return nil, err
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatusCode, CurrencyCode string
		Limit                                      int
	}{"management_credit_account", filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	offset, err := creditPageOffset(filter.Page, filter.Limit, filter.Cursor)
	if err != nil {
		return nil, err
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.store.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	query := creditAccountDirectorySelect
	arguments := make([]any, 0, 8)
	if filter.CustomerNo != "" {
		query += ` AND account.customer_no = ?`
		arguments = append(arguments, filter.CustomerNo)
	}
	if filter.StatusCode != "" {
		query += ` AND account.status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND account.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	filteredQuery, filteredArguments := query, append([]any(nil), arguments...)
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((account.updated_at < ?) OR (account.updated_at = ? AND account.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY account.updated_at DESC, account.id DESC LIMIT ? OFFSET ?`
	arguments = append(arguments, filter.Limit+1, offset)
	rows := make([]creditAccountDirectoryRow, 0, filter.Limit+1)
	total, err := readCreditDirectoryPage(ctx, s.store.database, &rows, filteredQuery, filteredArguments, query, arguments)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &CreditAccountDirectoryPage{Total: total, Accounts: make([]CreditAccountDirectoryItem, 0, len(rows))}
	for _, row := range rows {
		item, projectErr := projectCreditAccountDirectoryItem(row, true)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Accounts = append(page.Accounts, item)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.store.cursor.encode(directoryCursorBoundary{
			SortTimeMicroseconds: last.UpdatedAt.UnixMicro(), ID: last.ID,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
		}
	}
	return page, nil
}

func (s *managementCreditDirectoryImpl) GetManagementCreditAccount(
	ctx context.Context,
	creditAccountNo string,
) (*CreditAccountDirectoryItem, error) {
	creditAccountNo = strings.TrimSpace(creditAccountNo)
	if _, err := s.authorizer.AuthorizeAccountDetail(ctx, creditAccountNo); err != nil {
		return nil, err
	}
	if !validBusinessReference(creditAccountNo, 40) {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	var row *creditAccountDirectoryRow
	query := creditAccountDirectorySelect + ` AND account.credit_account_no = ?`
	if err := s.store.database.GetScan(ctx, &row, query, creditAccountNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	item, err := projectCreditAccountDirectoryItem(*row, true)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func projectCreditAccountDirectoryItem(
	row creditAccountDirectoryRow,
	management bool,
) (CreditAccountDirectoryItem, error) {
	if !validCreditAccountDirectoryRow(row) {
		return CreditAccountDirectoryItem{}, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	item := CreditAccountDirectoryItem{
		CreditAccountNo: row.CreditAccountNo, CurrencyCode: row.CurrencyCode,
		StatusCode: row.StatusCode, TotalLimitMinor: row.TotalLimitMinor,
		AvailableLimitMinor:      row.AvailableLimitMinor,
		UnbilledPrincipalMinor:   row.UnbilledPrincipalMinor,
		BilledPrincipalMinor:     row.BilledPrincipalMinor,
		DelinquentPrincipalMinor: row.DelinquentPrincipalMinor,
		WrittenOffPrincipalMinor: row.WrittenOffPrincipalMinor,
		ValidFrom:                row.ValidFrom.UTC(), ValidUntil: row.ValidUntil.UTC(),
		FreezeReasonCode: optionalDirectoryString(row.FreezeReasonCode), FrozenAt: row.FrozenAt,
		Version: row.Version, UpdatedAt: row.UpdatedAt.UTC(),
		Terms: CreditAccountTermsDirectoryItem{
			TermsNo: row.TermsNo, TermsVersion: row.TermsVersion,
			TotalLimitMinor: row.TermsTotalLimitMinor, BillingDay: row.BillingDay,
			RepaymentPeriodDays: row.RepaymentPeriodDays, GracePeriodDays: row.GracePeriodDays,
			DailyLateFeeBasisPoints: row.DailyLateFeeBasisPoints,
			MaximumLateFeeMinor:     row.MaximumLateFeeMinor,
			EffectiveFrom:           row.EffectiveFrom.UTC(), EffectiveUntil: row.EffectiveUntil.UTC(),
		},
	}
	if management {
		item.CustomerNo = row.CustomerNo
	}
	delinquency, err := projectCreditAccountDelinquency(row)
	if err != nil {
		return CreditAccountDirectoryItem{}, err
	}
	item.LatestDelinquency = delinquency
	return item, nil
}

func validCreditAccountDirectoryRow(row creditAccountDirectoryRow) bool {
	return validCreditAccountDirectoryIdentity(row) && validCreditAccountDirectoryTerms(row)
}

func validCreditAccountDirectoryIdentity(row creditAccountDirectoryRow) bool {
	total := row.AvailableLimitMinor + row.UnbilledPrincipalMinor +
		row.BilledPrincipalMinor + row.WrittenOffPrincipalMinor
	return row.ID > 0 && validBusinessReference(row.CreditAccountNo, 40) &&
		currencyPattern.MatchString(row.CurrencyCode) && row.TotalLimitMinor > 0 &&
		row.AvailableLimitMinor >= 0 && row.UnbilledPrincipalMinor >= 0 &&
		row.BilledPrincipalMinor >= 0 && row.DelinquentPrincipalMinor >= 0 &&
		row.WrittenOffPrincipalMinor >= 0 && row.DelinquentPrincipalMinor <= row.BilledPrincipalMinor &&
		total == row.TotalLimitMinor && row.Version > 0 && !row.ValidFrom.IsZero() &&
		row.ValidUntil.After(row.ValidFrom) && !row.UpdatedAt.IsZero()
}

func validCreditAccountDirectoryTerms(row creditAccountDirectoryRow) bool {
	return row.TermsVersion > 0 && row.TermsTotalLimitMinor == row.TotalLimitMinor &&
		row.BillingDay >= 1 && row.BillingDay <= 28 && row.RepaymentPeriodDays >= 1 &&
		row.GracePeriodDays >= 0 && row.DailyLateFeeBasisPoints >= 0 &&
		row.MaximumLateFeeMinor >= 0 && !row.EffectiveFrom.IsZero() &&
		row.EffectiveUntil.After(row.EffectiveFrom)
}

func projectCreditAccountDelinquency(
	row creditAccountDirectoryRow,
) (*CreditDelinquencyDirectoryItem, error) {
	if row.CaseNo == nil {
		return nil, nil
	}
	if row.CaseStatementNo == nil || row.CaseStatusCode == nil ||
		row.CasePrincipalMinor == nil || row.CaseLateFeeMinor == nil ||
		row.CaseOutstandingMinor == nil || row.CaseOpenedAt == nil ||
		row.CaseGraceUntil == nil || *row.CasePrincipalMinor < 0 ||
		*row.CaseLateFeeMinor < 0 || *row.CaseOutstandingMinor < 0 {
		return nil, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	return &CreditDelinquencyDirectoryItem{
		CaseNo: *row.CaseNo, StatementNo: *row.CaseStatementNo,
		StatusCode: *row.CaseStatusCode, PrincipalMinor: *row.CasePrincipalMinor,
		LateFeeMinor:           *row.CaseLateFeeMinor,
		OutstandingAmountMinor: *row.CaseOutstandingMinor,
		OpenedAt:               row.CaseOpenedAt.UTC(), GraceUntil: row.CaseGraceUntil.UTC(),
		ResolvedAt: row.CaseResolvedAt,
	}, nil
}
