// This file loads bounded credit-statement, line, and repayment projections.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditStatementDirectoryRow struct {
	ID                     int64      `orm:"id"`
	StatementNo            string     `orm:"statement_no"`
	CreditAccountNo        string     `orm:"credit_account_no"`
	CustomerNo             string     `orm:"customer_no"`
	CurrencyCode           string     `orm:"currency_code"`
	PeriodStartedAt        time.Time  `orm:"period_started_at"`
	PeriodEndedAt          time.Time  `orm:"period_ended_at"`
	LineCount              int        `orm:"line_count"`
	PrincipalMinor         int64      `orm:"principal_minor"`
	FeeMinor               int64      `orm:"fee_minor"`
	AdjustmentMinor        int64      `orm:"adjustment_minor"`
	TotalAmountMinor       int64      `orm:"total_amount_minor"`
	PaidAmountMinor        int64      `orm:"paid_amount_minor"`
	WrittenOffAmountMinor  int64      `orm:"written_off_amount_minor"`
	OutstandingAmountMinor int64      `orm:"outstanding_amount_minor"`
	StatusCode             string     `orm:"status_code"`
	IssuedAt               time.Time  `orm:"issued_at"`
	PaymentDueAt           time.Time  `orm:"payment_due_at"`
	OverdueAt              *time.Time `orm:"overdue_at"`
	SettledAt              *time.Time `orm:"settled_at"`
	WrittenOffAt           *time.Time `orm:"written_off_at"`
	Version                int64      `orm:"version"`
	AvailableBalanceMinor  int64      `orm:"available_balance_minor"`
	CreatedAt              time.Time  `orm:"created_at"`
	UpdatedAt              time.Time  `orm:"updated_at"`
}

type creditStatementLineDirectoryRow struct {
	ID              int64     `orm:"id"`
	StatementLineNo string    `orm:"statement_line_no"`
	LineNumber      int       `orm:"line_number"`
	LineTypeCode    string    `orm:"line_type_code"`
	SourceTypeCode  string    `orm:"source_type_code"`
	SourceReference string    `orm:"source_reference"`
	AmountMinor     int64     `orm:"amount_minor"`
	CreatedAt       time.Time `orm:"created_at"`
}

type creditRepaymentDirectoryRow struct {
	RepaymentNo            string     `orm:"repayment_no"`
	RepaymentTypeCode      string     `orm:"repayment_type_code"`
	FundingSourceTypeCode  string     `orm:"funding_source_type_code"`
	AmountMinor            int64      `orm:"amount_minor"`
	AllocatedAmountMinor   int64      `orm:"allocated_amount_minor"`
	UnallocatedAmountMinor int64      `orm:"unallocated_amount_minor"`
	StatusCode             string     `orm:"status_code"`
	ConfirmedAt            *time.Time `orm:"confirmed_at"`
	CreatedAt              time.Time  `orm:"created_at"`
}

type creditStatementDirectoryStatus string

const (
	creditStatementDirectoryStatusOpen          creditStatementDirectoryStatus = "open"
	creditStatementDirectoryStatusPartiallyPaid creditStatementDirectoryStatus = "partially_paid"
	creditStatementDirectoryStatusOverdue       creditStatementDirectoryStatus = "overdue"
)

const creditStatementDirectorySelect = `
SELECT statement.id, statement.statement_no, statement.credit_account_no,
       statement.customer_no, statement.currency_code,
       statement.period_started_at, statement.period_ended_at, statement.line_count,
       statement.principal_minor, statement.fee_minor, statement.adjustment_minor,
       statement.total_amount_minor, statement.paid_amount_minor,
       statement.written_off_amount_minor, statement.outstanding_amount_minor,
       statement.status_code, statement.issued_at, statement.payment_due_at,
       statement.overdue_at, statement.settled_at, statement.written_off_at,
       statement.version, %s AS available_balance_minor,
       statement.created_at, statement.updated_at
FROM plugin_idc_core_credit_statement statement
%s
WHERE statement.tenant_id = 0`

func (s *accountCreditDirectoryImpl) ListAccountCreditStatements(
	ctx context.Context,
	filter CreditStatementDirectoryFilter,
) (*CreditStatementDirectoryPage, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	filter.CustomerNo = audience.CustomerNo
	return s.store.listCreditStatements(ctx, filter, false)
}

func (s *managementCreditDirectoryImpl) ListManagementCreditStatements(
	ctx context.Context,
	filter CreditStatementDirectoryFilter,
) (*CreditStatementDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeStatementList(ctx); err != nil {
		return nil, err
	}
	return s.store.listCreditStatements(ctx, filter, true)
}

func (s *creditDirectoryStore) listCreditStatements(
	ctx context.Context,
	filter CreditStatementDirectoryFilter,
	management bool,
) (*CreditStatementDirectoryPage, error) {
	var err error
	filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Cursor, filter.Limit, err =
		normalizeCreditDirectoryFilter(
			filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Cursor, filter.Limit,
		)
	if err != nil {
		return nil, err
	}
	kind := "account_credit_statement"
	offset, err := creditPageOffset(filter.Page, filter.Limit, filter.Cursor)
	if err != nil {
		return nil, err
	}
	if management {
		kind = "management_credit_statement"
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatusCode, CurrencyCode string
		Limit                                      int
	}{kind, filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	availableExpression := "0"
	join := ""
	arguments := make([]any, 0, 10)
	if !management {
		availableExpression = "COALESCE(balance.available_balance_minor, 0)"
		join = `
LEFT JOIN LATERAL (
    SELECT COALESCE(snapshot.closing_balance_minor, 0)
             - COALESCE(hold.held_balance_minor, 0) AS available_balance_minor
    FROM plugin_idc_core_billing_ledger_account account
    LEFT JOIN LATERAL (
        SELECT closing_balance_minor
        FROM plugin_idc_core_billing_balance_snapshot
        WHERE tenant_id = 0 AND account_no = account.account_no
        ORDER BY sequence_no DESC LIMIT 1
    ) snapshot ON TRUE
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(amount_minor - consumed_amount_minor - released_amount_minor), 0)
               AS held_balance_minor
        FROM plugin_idc_core_billing_fund_hold
        WHERE tenant_id = 0 AND account_no = account.account_no
          AND status_code IN ('active', 'partially_consumed')
          AND expires_at > CURRENT_TIMESTAMP
    ) hold ON TRUE
    WHERE account.tenant_id = 0 AND account.customer_no = statement.customer_no
      AND account.currency_code = statement.currency_code
      AND account.account_type_code = 'customer_balance'
    LIMIT 1
) balance ON TRUE`
	}
	query := strings.Replace(creditStatementDirectorySelect, "%s", availableExpression, 1)
	query = strings.Replace(query, "%s", join, 1)
	if filter.CustomerNo != "" {
		query += ` AND statement.customer_no = ?`
		arguments = append(arguments, filter.CustomerNo)
	}
	if filter.StatusCode != "" {
		query += ` AND statement.status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND statement.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	filteredQuery, filteredArguments := query, append([]any(nil), arguments...)
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((statement.issued_at < ?) OR (statement.issued_at = ? AND statement.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY statement.issued_at DESC, statement.id DESC LIMIT ? OFFSET ?`
	arguments = append(arguments, filter.Limit+1, offset)
	rows := make([]creditStatementDirectoryRow, 0, filter.Limit+1)
	total, err := readCreditDirectoryPage(ctx, s.database, &rows, filteredQuery, filteredArguments, query, arguments)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &CreditStatementDirectoryPage{Total: total, Statements: make([]CreditStatementDirectoryItem, 0, len(rows))}
	for _, row := range rows {
		item, projectErr := projectCreditStatementDirectoryItem(row, management)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Statements = append(page.Statements, item)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
			SortTimeMicroseconds: last.IssuedAt.UnixMicro(), ID: last.ID,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
		}
	}
	return page, nil
}

func (s *accountCreditDirectoryImpl) GetAccountCreditStatement(
	ctx context.Context,
	statementNo string,
	filter CreditStatementLineDirectoryFilter,
) (*CreditStatementDirectoryDetail, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.getCreditStatement(ctx, statementNo, audience.CustomerNo, filter, false)
}

func (s *managementCreditDirectoryImpl) GetManagementCreditStatement(
	ctx context.Context,
	statementNo string,
	filter CreditStatementLineDirectoryFilter,
) (*CreditStatementDirectoryDetail, error) {
	statementNo = strings.TrimSpace(statementNo)
	if _, err := s.authorizer.AuthorizeStatementDetail(ctx, statementNo); err != nil {
		return nil, err
	}
	return s.store.getCreditStatement(ctx, statementNo, "", filter, true)
}

func (s *creditDirectoryStore) getCreditStatement(
	ctx context.Context,
	statementNo string,
	customerNo string,
	filter CreditStatementLineDirectoryFilter,
	management bool,
) (*CreditStatementDirectoryDetail, error) {
	statementNo = strings.TrimSpace(statementNo)
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	var err error
	filter.Limit, err = normalizeDirectoryLimit(
		filter.Limit, defaultCreditStatementLineLimit, maximumCreditStatementLineLimit,
	)
	if err != nil || !validBusinessReference(statementNo, 40) ||
		len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	kind := "account_credit_statement_lines"
	if management {
		kind = "management_credit_statement_lines"
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatementNo string
		Limit                         int
	}{kind, customerNo, statementNo, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	result := &CreditStatementDirectoryDetail{
		Lines: []CreditStatementLineDirectoryItem{}, Repayments: []CreditRepaymentDirectoryItem{},
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		row, projected, loadErr := loadCreditStatementDirectoryRoot(
			tx, statementNo, customerNo, management,
		)
		if loadErr != nil {
			return loadErr
		}
		result.Statement = projected
		result.Lines, result.NextLineCursor, loadErr = s.loadCreditStatementLines(
			tx, statementNo, row.CustomerNo, filter.Limit, boundary, digest,
		)
		if loadErr != nil {
			return loadErr
		}
		result.Repayments, result.RepaymentsTruncated, loadErr = loadCreditStatementRepayments(
			tx, statementNo, row.CustomerNo,
		)
		if loadErr != nil {
			return loadErr
		}
		return nil
	})
	if err != nil {
		return nil, creditDirectoryResultError(err)
	}
	return result, nil
}

func loadCreditStatementDirectoryRoot(
	tx gdb.TX,
	statementNo string,
	customerNo string,
	management bool,
) (*creditStatementDirectoryRow, CreditStatementDirectoryItem, error) {
	availableExpression := "0"
	join := ""
	arguments := []any{statementNo}
	if !management {
		availableExpression = "COALESCE(balance.available_balance_minor, 0)"
		join = `
LEFT JOIN LATERAL (
    SELECT COALESCE(snapshot.closing_balance_minor, 0)
             - COALESCE(hold.held_balance_minor, 0) AS available_balance_minor
    FROM plugin_idc_core_billing_ledger_account account
    LEFT JOIN LATERAL (
        SELECT closing_balance_minor
        FROM plugin_idc_core_billing_balance_snapshot
        WHERE tenant_id = 0 AND account_no = account.account_no
        ORDER BY sequence_no DESC LIMIT 1
    ) snapshot ON TRUE
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(amount_minor - consumed_amount_minor - released_amount_minor), 0)
               AS held_balance_minor
        FROM plugin_idc_core_billing_fund_hold
        WHERE tenant_id = 0 AND account_no = account.account_no
          AND status_code IN ('active', 'partially_consumed')
          AND expires_at > CURRENT_TIMESTAMP
    ) hold ON TRUE
    WHERE account.tenant_id = 0 AND account.customer_no = statement.customer_no
      AND account.currency_code = statement.currency_code
      AND account.account_type_code = 'customer_balance'
    LIMIT 1
) balance ON TRUE`
	}
	query := strings.Replace(creditStatementDirectorySelect, "%s", availableExpression, 1)
	query = strings.Replace(query, "%s", join, 1)
	query += ` AND statement.statement_no = ?`
	if customerNo != "" {
		query += ` AND statement.customer_no = ?`
		arguments = append(arguments, customerNo)
	}
	var row *creditStatementDirectoryRow
	if err := tx.GetScan(&row, query, arguments...); err != nil {
		return nil, CreditStatementDirectoryItem{}, gerror.Wrap(err, "load credit statement detail")
	}
	if row == nil {
		return nil, CreditStatementDirectoryItem{}, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	projected, err := projectCreditStatementDirectoryItem(*row, management)
	if err != nil {
		return nil, CreditStatementDirectoryItem{}, err
	}
	return row, projected, nil
}

func (s *creditDirectoryStore) loadCreditStatementLines(
	tx gdb.TX,
	statementNo string,
	customerNo string,
	limit int,
	boundary *directoryCursorBoundary,
	digest [32]byte,
) ([]CreditStatementLineDirectoryItem, string, error) {
	query := `
SELECT id, statement_line_no, line_number, line_type_code, source_type_code,
       source_reference, amount_minor, created_at
FROM plugin_idc_core_credit_statement_line
WHERE tenant_id = 0 AND statement_no = ? AND customer_no = ?`
	arguments := []any{statementNo, customerNo}
	if boundary != nil {
		query += ` AND ((line_number > ?) OR (line_number = ? AND id > ?))`
		arguments = append(arguments, boundary.SortTimeMicroseconds, boundary.SortTimeMicroseconds, boundary.ID)
	}
	query += ` ORDER BY line_number ASC, id ASC LIMIT ?`
	arguments = append(arguments, limit+1)
	rows := make([]creditStatementLineDirectoryRow, 0, limit+1)
	if err := tx.GetScan(&rows, query, arguments...); err != nil {
		return nil, "", gerror.Wrap(err, "load credit statement lines")
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	result := make([]CreditStatementLineDirectoryItem, 0, len(rows))
	for _, row := range rows {
		if row.ID < 1 || row.LineNumber < 1 || row.LineNumber > 10000 ||
			row.StatementLineNo == "" || row.AmountMinor == 0 || row.CreatedAt.IsZero() {
			return nil, "", bizerr.NewCode(CodeDirectoryIntegrity)
		}
		result = append(result, CreditStatementLineDirectoryItem{
			StatementLineNo: row.StatementLineNo, LineNumber: row.LineNumber,
			LineTypeCode: row.LineTypeCode, SourceTypeCode: row.SourceTypeCode,
			SourceReference: row.SourceReference, AmountMinor: row.AmountMinor,
			CreatedAt: row.CreatedAt.UTC(),
		})
	}
	if !hasMore || len(rows) == 0 {
		return result, "", nil
	}
	last := rows[len(rows)-1]
	nextCursor, err := s.cursor.encode(directoryCursorBoundary{
		SortTimeMicroseconds: int64(last.LineNumber), ID: last.ID,
	}, digest)
	return result, nextCursor, err
}

func loadCreditStatementRepayments(
	tx gdb.TX,
	statementNo string,
	customerNo string,
) ([]CreditRepaymentDirectoryItem, bool, error) {
	rows := make([]creditRepaymentDirectoryRow, 0, maximumCreditRepaymentPreview)
	if err := tx.GetScan(&rows, `
SELECT repayment.repayment_no, repayment.repayment_type_code,
       repayment.funding_source_type_code, repayment.amount_minor,
       repayment.allocated_amount_minor, repayment.unallocated_amount_minor,
       repayment.status_code, repayment.confirmed_at, repayment.created_at
FROM plugin_idc_core_credit_repayment repayment
WHERE repayment.tenant_id = 0 AND repayment.customer_no = ?
  AND EXISTS (
      SELECT 1 FROM plugin_idc_core_credit_repayment_allocation allocation
      WHERE allocation.tenant_id = 0 AND allocation.repayment_no = repayment.repayment_no
        AND allocation.statement_no = ?
  )
ORDER BY repayment.created_at DESC, repayment.id DESC LIMIT ?`,
		customerNo, statementNo, maximumCreditRepaymentPreview+1); err != nil {
		return nil, false, gerror.Wrap(err, "load credit statement repayments")
	}
	truncated := len(rows) > maximumCreditRepaymentPreview
	if truncated {
		rows = rows[:maximumCreditRepaymentPreview]
	}
	result := make([]CreditRepaymentDirectoryItem, 0, len(rows))
	for _, row := range rows {
		if row.RepaymentNo == "" || row.AmountMinor <= 0 ||
			row.AllocatedAmountMinor < 0 || row.UnallocatedAmountMinor < 0 ||
			row.AllocatedAmountMinor+row.UnallocatedAmountMinor > row.AmountMinor ||
			row.CreatedAt.IsZero() {
			return nil, false, bizerr.NewCode(CodeDirectoryIntegrity)
		}
		result = append(result, CreditRepaymentDirectoryItem{
			RepaymentNo: row.RepaymentNo, RepaymentTypeCode: row.RepaymentTypeCode,
			FundingSourceTypeCode: row.FundingSourceTypeCode,
			AmountMinor:           row.AmountMinor, AllocatedAmountMinor: row.AllocatedAmountMinor,
			UnallocatedAmountMinor: row.UnallocatedAmountMinor,
			StatusCode:             row.StatusCode, ConfirmedAt: row.ConfirmedAt,
			CreatedAt: row.CreatedAt.UTC(),
		})
	}
	return result, truncated, nil
}

func projectCreditStatementDirectoryItem(
	row creditStatementDirectoryRow,
	management bool,
) (CreditStatementDirectoryItem, error) {
	if row.ID < 1 || !validBusinessReference(row.StatementNo, 40) ||
		!validBusinessReference(row.CreditAccountNo, 40) ||
		!currencyPattern.MatchString(row.CurrencyCode) || row.LineCount < 1 || row.LineCount > 10000 ||
		row.PrincipalMinor < 0 || row.FeeMinor < 0 || row.TotalAmountMinor < 0 ||
		row.PaidAmountMinor < 0 || row.WrittenOffAmountMinor < 0 ||
		row.OutstandingAmountMinor < 0 ||
		row.PrincipalMinor+row.FeeMinor+row.AdjustmentMinor != row.TotalAmountMinor ||
		row.PaidAmountMinor+row.WrittenOffAmountMinor+row.OutstandingAmountMinor != row.TotalAmountMinor ||
		row.AvailableBalanceMinor < 0 || !row.PeriodEndedAt.After(row.PeriodStartedAt) ||
		row.IssuedAt.IsZero() || !row.PaymentDueAt.After(row.IssuedAt) ||
		row.Version < 1 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return CreditStatementDirectoryItem{}, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	status := creditStatementDirectoryStatus(row.StatusCode)
	payable := status == creditStatementDirectoryStatusOpen ||
		status == creditStatementDirectoryStatusPartiallyPaid ||
		status == creditStatementDirectoryStatusOverdue
	item := CreditStatementDirectoryItem{
		StatementNo: row.StatementNo, CreditAccountNo: row.CreditAccountNo,
		CurrencyCode: row.CurrencyCode, PeriodStartedAt: row.PeriodStartedAt.UTC(),
		PeriodEndedAt: row.PeriodEndedAt.UTC(), LineCount: row.LineCount,
		PrincipalMinor: row.PrincipalMinor, FeeMinor: row.FeeMinor,
		AdjustmentMinor: row.AdjustmentMinor, TotalAmountMinor: row.TotalAmountMinor,
		PaidAmountMinor: row.PaidAmountMinor, WrittenOffAmountMinor: row.WrittenOffAmountMinor,
		OutstandingAmountMinor: row.OutstandingAmountMinor, StatusCode: row.StatusCode,
		IssuedAt: row.IssuedAt.UTC(), PaymentDueAt: row.PaymentDueAt.UTC(),
		OverdueAt: row.OverdueAt, SettledAt: row.SettledAt, WrittenOffAt: row.WrittenOffAt,
		Version: row.Version, AvailableBalanceMinor: row.AvailableBalanceMinor,
		CanRepayFromBalance: !management && payable && row.OutstandingAmountMinor > 0 && row.AvailableBalanceMinor > 0,
		CreatedAt:           row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
	if management {
		item.CustomerNo = row.CustomerNo
	}
	return item, nil
}
