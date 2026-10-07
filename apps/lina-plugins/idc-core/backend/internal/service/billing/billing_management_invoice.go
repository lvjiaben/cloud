package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customerprojection"
)

func (s *managementFinanceDirectoryImpl) ListManagementInvoices(
	ctx context.Context,
	filter ManagementInvoiceDirectoryFilter,
) (*InvoiceDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeInvoiceList(ctx); err != nil {
		return nil, err
	}
	var err error
	filter.CustomerNo, err = normalizeManagementReference(filter.CustomerNo)
	if err != nil {
		return nil, err
	}
	filter.StatusCode, err = normalizeDirectoryStatus(filter.StatusCode)
	if err != nil {
		return nil, err
	}
	filter.CurrencyCode, err = normalizeDirectoryCurrency(filter.CurrencyCode)
	if err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(filter.Limit, defaultAccountInvoiceLimit, maximumManagementDirectoryLimit)
	if err != nil || len(filter.Cursor) > maximumDirectoryCursorLength || !validManagementPage(filter.Page, filter.Limit, filter.Cursor) {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatusCode, CurrencyCode string
		Limit                                      int
	}{"management_invoice", filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Limit})
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
SELECT invoice.id, invoice.invoice_no, invoice.customer_no, invoice.order_no,
       invoice.source_code, invoice.currency_code, invoice.status_code,
       invoice.item_count, invoice.total_amount_minor, invoice.paid_amount_minor,
       invoice.waived_amount_minor, invoice.credit_amount_minor,
       invoice.refundable_amount_minor,
       invoice.refunded_amount_minor, 0 AS available_balance_minor,
       invoice.payment_due_at, invoice.issued_at, invoice.settled_at, invoice.created_at
FROM plugin_idc_core_billing_invoice invoice
WHERE invoice.tenant_id = 0`
	arguments := make([]any, 0, 8)
	if filter.CustomerNo != "" {
		query += ` AND invoice.customer_no = ?`
		arguments = append(arguments, filter.CustomerNo)
	}
	if filter.StatusCode != "" {
		query += ` AND invoice.status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND invoice.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	countQuery, countArgs := "SELECT COUNT(*) FROM ("+query+") eligible", append([]any(nil), arguments...)
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((invoice.created_at < ?) OR (invoice.created_at = ? AND invoice.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY invoice.created_at DESC, invoice.id DESC LIMIT ? OFFSET ?`
	arguments = append(arguments, filter.Limit+1, max(filter.Page-1, 0)*filter.Limit)
	rows := make([]invoiceDirectoryRow, 0, filter.Limit+1)
	total, err := s.readManagementDirectoryPage(ctx, &rows, countQuery, countArgs, query, arguments)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &InvoiceDirectoryPage{Invoices: make([]InvoiceDirectoryItem, 0, len(rows)), Total: total}
	for _, row := range rows {
		item, projectErr := projectInvoiceDirectoryItem(row, true)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Invoices = append(page.Invoices, item)
	}
	customerNos := make([]string, 0, len(page.Invoices))
	for _, item := range page.Invoices {
		customerNos = append(customerNos, item.CustomerNo)
	}
	customers, err := customerprojection.Load(ctx, s.database, customerNos)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	for index := range page.Invoices {
		customer := customers[page.Invoices[index].CustomerNo]
		page.Invoices[index].CustomerCode = customer.CustomerCode
		page.Invoices[index].CustomerDisplayName = customer.DisplayName
		page.Invoices[index].CustomerPrimaryPhone = customer.PrimaryPhone
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

func (s *managementFinanceDirectoryImpl) GetManagementInvoice(
	ctx context.Context,
	invoiceNo string,
	filter InvoiceItemDirectoryFilter,
) (*InvoiceDirectoryDetail, error) {
	invoiceNo = strings.TrimSpace(invoiceNo)
	if _, err := s.authorizer.AuthorizeInvoiceDetail(ctx, invoiceNo); err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	var err error
	filter.Limit, err = normalizeDirectoryLimit(filter.Limit, defaultInvoiceItemLimit, maximumInvoiceItemLimit)
	if err != nil || !validBusinessReference(invoiceNo, 40) || len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, InvoiceNo string
		Limit           int
	}{"management_invoice_items", invoiceNo, filter.Limit})
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
SELECT id, invoice_no, customer_no, order_no, source_code, currency_code,
       status_code, item_count, total_amount_minor, paid_amount_minor,
       waived_amount_minor, credit_amount_minor, refundable_amount_minor,
       refunded_amount_minor,
       0 AS available_balance_minor, payment_due_at, issued_at, settled_at, created_at
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND invoice_no = ?`, invoiceNo); loadErr != nil {
			return gerror.Wrap(loadErr, "load management invoice detail")
		}
		if row == nil {
			return bizerr.NewCode(CodeDirectoryNotAccessible)
		}
		projected, projectErr := projectInvoiceDirectoryItem(*row, true)
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
		arguments := []any{invoiceNo, row.CustomerNo}
		if boundary != nil {
			query += ` AND ((line_number > ?) OR (line_number = ? AND id > ?))`
			arguments = append(arguments, boundary.SortTimeMicroseconds, boundary.SortTimeMicroseconds, boundary.ID)
		}
		query += ` ORDER BY line_number ASC, id ASC LIMIT ?`
		arguments = append(arguments, filter.Limit+1)
		rows := make([]invoiceItemDirectoryRow, 0, filter.Limit+1)
		if loadErr := tx.GetScan(&rows, query, arguments...); loadErr != nil {
			return gerror.Wrap(loadErr, "load management invoice items")
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
			result.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
				SortTimeMicroseconds: int64(last.LineNumber), ID: last.ID,
			}, digest)
			return err
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	customers, projectionErr := customerprojection.Load(ctx, s.database, []string{result.Invoice.CustomerNo})
	if projectionErr != nil {
		return nil, bizerr.WrapCode(projectionErr, CodeDirectoryUnavailable)
	}
	customer := customers[result.Invoice.CustomerNo]
	result.Invoice.CustomerCode = customer.CustomerCode
	result.Invoice.CustomerDisplayName = customer.DisplayName
	result.Invoice.CustomerPrimaryPhone = customer.PrimaryPhone
	return result, nil
}
