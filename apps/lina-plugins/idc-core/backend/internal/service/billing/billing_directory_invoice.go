package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type invoiceDirectoryRow struct {
	ID                    int64      `orm:"id"`
	InvoiceNo             string     `orm:"invoice_no"`
	CustomerNo            string     `orm:"customer_no"`
	OrderNo               *string    `orm:"order_no"`
	SourceCode            string     `orm:"source_code"`
	CurrencyCode          string     `orm:"currency_code"`
	StatusCode            string     `orm:"status_code"`
	ItemCount             int        `orm:"item_count"`
	TotalAmountMinor      int64      `orm:"total_amount_minor"`
	PaidAmountMinor       int64      `orm:"paid_amount_minor"`
	WaivedAmountMinor     int64      `orm:"waived_amount_minor"`
	CreditAmountMinor     int64      `orm:"credit_amount_minor"`
	RefundableAmountMinor int64      `orm:"refundable_amount_minor"`
	RefundedAmountMinor   int64      `orm:"refunded_amount_minor"`
	AvailableBalanceMinor int64      `orm:"available_balance_minor"`
	PaymentDueAt          time.Time  `orm:"payment_due_at"`
	IssuedAt              *time.Time `orm:"issued_at"`
	SettledAt             *time.Time `orm:"settled_at"`
	CreatedAt             time.Time  `orm:"created_at"`
}

type invoiceItemDirectoryRow struct {
	ID               int64  `orm:"id"`
	InvoiceItemNo    string `orm:"invoice_item_no"`
	LineNumber       int    `orm:"line_number"`
	ItemTypeCode     string `orm:"item_type_code"`
	SourceReference  string `orm:"source_reference"`
	DescriptionZhCN  string `orm:"description_zh_cn"`
	DescriptionEnUS  string `orm:"description_en_us"`
	SubtotalMinor    int64  `orm:"subtotal_minor"`
	DiscountMinor    int64  `orm:"discount_minor"`
	NetAmountMinor   int64  `orm:"net_amount_minor"`
	TaxAmountMinor   int64  `orm:"tax_amount_minor"`
	TotalAmountMinor int64  `orm:"total_amount_minor"`
}

func (s *financeDirectoryImpl) ListAccountInvoices(
	ctx context.Context,
	filter InvoiceDirectoryFilter,
) (*InvoiceDirectoryPage, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	filter.CustomerNo = customerNo
	filter.StatusCode, err = normalizeDirectoryStatus(filter.StatusCode)
	if err != nil {
		return nil, err
	}
	filter.CurrencyCode, err = normalizeDirectoryCurrency(filter.CurrencyCode)
	if err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(
		filter.Limit, defaultAccountInvoiceLimit, maximumAccountInvoiceLimit,
	)
	if err != nil || len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatusCode, CurrencyCode string
		Limit                                      int
	}{"account_invoice", customerNo, filter.StatusCode, filter.CurrencyCode, filter.Limit})
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
	query := `
WITH customer_balance AS (
    SELECT account.currency_code,
           COALESCE(snapshot.closing_balance_minor, 0)
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
    WHERE account.tenant_id = 0 AND account.customer_no = ?
      AND account.account_type_code = 'customer_balance'
)
SELECT invoice.id, invoice.invoice_no, invoice.customer_no, invoice.order_no,
       invoice.source_code, invoice.currency_code, invoice.status_code,
       invoice.item_count, invoice.total_amount_minor, invoice.paid_amount_minor,
       invoice.waived_amount_minor, invoice.credit_amount_minor,
       invoice.refundable_amount_minor,
       invoice.refunded_amount_minor,
       COALESCE(customer_balance.available_balance_minor, 0) AS available_balance_minor,
       invoice.payment_due_at, invoice.issued_at, invoice.settled_at, invoice.created_at
FROM plugin_idc_core_billing_invoice invoice
LEFT JOIN customer_balance ON customer_balance.currency_code = invoice.currency_code
WHERE invoice.tenant_id = 0 AND invoice.customer_no = ?`
	arguments := []any{customerNo, customerNo}
	if filter.StatusCode != "" {
		query += ` AND invoice.status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND invoice.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((invoice.created_at < ?) OR (invoice.created_at = ? AND invoice.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY invoice.created_at DESC, invoice.id DESC LIMIT ?`
	arguments = append(arguments, filter.Limit+1)
	rows := make([]invoiceDirectoryRow, 0, filter.Limit+1)
	if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &InvoiceDirectoryPage{Invoices: make([]InvoiceDirectoryItem, 0, len(rows))}
	for _, row := range rows {
		item, projectErr := projectInvoiceDirectoryItem(row, false)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Invoices = append(page.Invoices, item)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
			SortTimeMicroseconds: last.CreatedAt.UnixMicro(), ID: last.ID,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
		}
	}
	return page, nil
}

func (s *financeDirectoryImpl) GetAccountInvoice(
	ctx context.Context,
	invoiceNo string,
	filter InvoiceItemDirectoryFilter,
) (*InvoiceDirectoryDetail, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	invoiceNo = strings.TrimSpace(invoiceNo)
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(filter.Limit, defaultInvoiceItemLimit, maximumInvoiceItemLimit)
	if err != nil || !validBusinessReference(invoiceNo, 40) || len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, InvoiceNo string
		Limit                       int
	}{"account_invoice_items", customerNo, invoiceNo, filter.Limit})
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
	result := &InvoiceDirectoryDetail{Items: []InvoiceItemDirectoryItem{}}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		var row *invoiceDirectoryRow
		if loadErr := tx.GetScan(&row, `
SELECT invoice.id, invoice.invoice_no, invoice.customer_no, invoice.order_no,
       invoice.source_code, invoice.currency_code, invoice.status_code,
       invoice.item_count, invoice.total_amount_minor, invoice.paid_amount_minor,
       invoice.waived_amount_minor, invoice.credit_amount_minor,
       invoice.refundable_amount_minor,
       invoice.refunded_amount_minor,
       COALESCE(balance.available_balance_minor, 0) AS available_balance_minor,
       invoice.payment_due_at, invoice.issued_at, invoice.settled_at, invoice.created_at
FROM plugin_idc_core_billing_invoice invoice
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
    WHERE account.tenant_id = 0 AND account.customer_no = invoice.customer_no
      AND account.currency_code = invoice.currency_code
      AND account.account_type_code = 'customer_balance'
    LIMIT 1
) balance ON TRUE
WHERE invoice.tenant_id = 0 AND invoice.invoice_no = ? AND invoice.customer_no = ?`,
			invoiceNo, customerNo); loadErr != nil {
			return gerror.Wrap(loadErr, "load customer invoice detail")
		}
		if row == nil {
			return bizerr.NewCode(CodeDirectoryNotAccessible)
		}
		projected, projectErr := projectInvoiceDirectoryItem(*row, false)
		if projectErr != nil {
			return projectErr
		}
		result.Invoice = projected
		query := `
SELECT id, invoice_item_no, line_number, item_type_code, source_reference,
       description_zh_cn, description_en_us, subtotal_minor, discount_minor,
       net_amount_minor, tax_amount_minor, total_amount_minor
FROM plugin_idc_core_billing_invoice_item
WHERE tenant_id = 0 AND invoice_no = ? AND customer_no = ?`
		arguments := []any{invoiceNo, customerNo}
		if boundary != nil {
			query += ` AND ((line_number > ?) OR (line_number = ? AND id > ?))`
			arguments = append(arguments, boundary.SortTimeMicroseconds, boundary.SortTimeMicroseconds, boundary.ID)
		}
		query += ` ORDER BY line_number ASC, id ASC LIMIT ?`
		arguments = append(arguments, filter.Limit+1)
		rows := make([]invoiceItemDirectoryRow, 0, filter.Limit+1)
		if loadErr := tx.GetScan(&rows, query, arguments...); loadErr != nil {
			return gerror.Wrap(loadErr, "load customer invoice items")
		}
		hasMore := len(rows) > filter.Limit
		if hasMore {
			rows = rows[:filter.Limit]
		}
		for _, item := range rows {
			result.Items = append(result.Items, InvoiceItemDirectoryItem{
				InvoiceItemNo: item.InvoiceItemNo, LineNumber: item.LineNumber,
				ItemTypeCode: item.ItemTypeCode, SourceReference: item.SourceReference,
				DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
				SubtotalMinor: item.SubtotalMinor, DiscountMinor: item.DiscountMinor,
				NetAmountMinor: item.NetAmountMinor, TaxAmountMinor: item.TaxAmountMinor,
				TotalAmountMinor: item.TotalAmountMinor,
			})
		}
		if hasMore && len(rows) > 0 {
			last := rows[len(rows)-1]
			encoded, encodeErr := s.cursor.encode(directoryCursorBoundary{
				SortTimeMicroseconds: int64(last.LineNumber), ID: last.ID,
			}, digest)
			if encodeErr != nil {
				return encodeErr
			}
			result.NextCursor = encoded
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeDirectoryInvalid) || bizerr.Is(err, CodeDirectoryNotAccessible) ||
			bizerr.Is(err, CodeDirectoryIntegrity) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	return result, nil
}

func projectInvoiceDirectoryItem(
	row invoiceDirectoryRow,
	management bool,
) (InvoiceDirectoryItem, error) {
	outstanding := row.TotalAmountMinor - row.PaidAmountMinor -
		row.WaivedAmountMinor - row.CreditAmountMinor
	if row.ID < 1 || row.CreatedAt.IsZero() || row.PaymentDueAt.IsZero() ||
		row.TotalAmountMinor < 0 || row.PaidAmountMinor < 0 || row.WaivedAmountMinor < 0 ||
		row.CreditAmountMinor < 0 ||
		outstanding < 0 || row.AvailableBalanceMinor < 0 {
		return InvoiceDirectoryItem{}, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	payable := row.StatusCode == string(InvoiceStatusPendingPayment) ||
		row.StatusCode == string(InvoiceStatusPartiallyPaid) || row.StatusCode == string(InvoiceStatusOverdue)
	result := InvoiceDirectoryItem{
		InvoiceNo: row.InvoiceNo, OrderNo: optionalDirectoryString(row.OrderNo),
		SourceCode: row.SourceCode, CurrencyCode: row.CurrencyCode, StatusCode: row.StatusCode,
		ItemCount: row.ItemCount, TotalAmountMinor: row.TotalAmountMinor,
		PaidAmountMinor: row.PaidAmountMinor, WaivedAmountMinor: row.WaivedAmountMinor,
		CreditAmountMinor:      row.CreditAmountMinor,
		OutstandingAmountMinor: outstanding, RefundableAmountMinor: row.RefundableAmountMinor,
		RefundedAmountMinor: row.RefundedAmountMinor, PaymentDueAt: row.PaymentDueAt.UTC(),
		IssuedAt: row.IssuedAt, SettledAt: row.SettledAt, CreatedAt: row.CreatedAt.UTC(),
		CanPayFromBalance: payable && outstanding > 0 && row.AvailableBalanceMinor >= outstanding,
		CanMerge: payable && row.PaidAmountMinor == 0 && row.WaivedAmountMinor == 0 &&
			row.CreditAmountMinor == 0 &&
			row.RefundableAmountMinor == 0 && row.RefundedAmountMinor == 0,
	}
	if management {
		result.CustomerNo = row.CustomerNo
	}
	return result, nil
}
