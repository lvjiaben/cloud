// This file implements the bounded customer-owned tax-invoice request directory.

package taxinvoicerequest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	defaultCurrentCustomerLimit = 20
	maximumCurrentCustomerLimit = 50
)

type currentCustomerCursor struct {
	SubmittedAt time.Time `json:"submittedAt"`
	ID          int64     `json:"id"`
}

type currentCustomerRow struct {
	ID                 int64      `orm:"id"`
	TaxInvoiceNo       string     `orm:"tax_invoice_no"`
	StatusCode         string     `orm:"status_code"`
	InvoiceTypeCode    string     `orm:"invoice_type_code"`
	CurrencyCode       string     `orm:"tax_currency_code"`
	GrossAmountMinor   int64      `orm:"gross_amount_minor"`
	MaskedInvoiceNo    string     `orm:"masked_invoice_number"`
	DecisionReasonCode string     `orm:"decision_reason_code"`
	SubmittedAt        time.Time  `orm:"submitted_at"`
	IssuedAt           *time.Time `orm:"issued_at"`
	DeliveredAt        *time.Time `orm:"delivered_at"`
	Version            int64      `orm:"version"`
}

// ListCurrentCustomerRequests returns one stable keyset page after deriving
// customer ownership exclusively from the authenticated request context.
func (s *serviceImpl) ListCurrentCustomerRequests(
	ctx context.Context,
	query CurrentCustomerQuery,
) (*CurrentCustomerPage, error) {
	if s == nil || nilDependency(s.customers) || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	limit := query.Limit
	if limit == 0 {
		limit = defaultCurrentCustomerLimit
	}
	if limit < 1 || limit > maximumCurrentCustomerLimit {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validBusinessNo(subject.CustomerNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	cursor, err := decodeCurrentCustomerCursor(query.Cursor)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	arguments := []any{idccap.DefaultTenantID, subject.CustomerNo}
	cursorPredicate := ""
	if cursor != nil {
		cursorPredicate = " AND (submitted_at, id) < (?, ?)"
		arguments = append(arguments, cursor.SubmittedAt.UTC(), cursor.ID)
	}
	arguments = append(arguments, limit+1)
	var rows []currentCustomerRow
	err = s.database.GetScan(ctx, &rows, `SELECT id, tax_invoice_no, status_code,
invoice_type_code, tax_currency_code, gross_amount_minor, masked_invoice_number,
decision_reason_code, submitted_at, issued_at, delivered_at, version
FROM plugin_idc_compliance_tax_invoice
WHERE tenant_id = ? AND customer_no = ? AND submitted_at IS NOT NULL`+cursorPredicate+`
ORDER BY submitted_at DESC, id DESC
LIMIT ?`, arguments...)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := &CurrentCustomerPage{Items: make([]CurrentCustomerRequest, 0, len(rows))}
	for index := range rows {
		item, projectErr := projectCurrentCustomerRow(&rows[index])
		if projectErr != nil {
			return nil, bizerr.WrapCode(projectErr, CodeUnavailable)
		}
		page.Items = append(page.Items, *item)
	}
	if hasMore && len(rows) > 0 {
		page.NextCursor, err = encodeCurrentCustomerCursor(currentCustomerCursor{
			SubmittedAt: rows[len(rows)-1].SubmittedAt,
			ID:          rows[len(rows)-1].ID,
		})
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
	}
	return page, nil
}

func projectCurrentCustomerRow(row *currentCustomerRow) (*CurrentCustomerRequest, error) {
	if row == nil || row.ID < 1 || !validBusinessNo(row.TaxInvoiceNo, 64) ||
		row.StatusCode == "" || row.InvoiceTypeCode == "" || len(row.CurrencyCode) != 3 ||
		row.GrossAmountMinor <= 0 || row.SubmittedAt.IsZero() || row.Version < 1 {
		return nil, gerror.New("customer tax invoice projection is invalid")
	}
	item := &CurrentCustomerRequest{
		TaxInvoiceNo: row.TaxInvoiceNo, StatusCode: row.StatusCode,
		InvoiceTypeCode: row.InvoiceTypeCode, CurrencyCode: row.CurrencyCode,
		GrossAmountMinor: row.GrossAmountMinor, MaskedInvoiceNo: row.MaskedInvoiceNo,
		DecisionReasonCode: row.DecisionReasonCode, SubmittedAt: row.SubmittedAt.UTC(),
		Version: row.Version,
	}
	if row.IssuedAt != nil {
		issued := row.IssuedAt.UTC()
		item.IssuedAt = &issued
	}
	if row.DeliveredAt != nil {
		delivered := row.DeliveredAt.UTC()
		item.DeliveredAt = &delivered
	}
	return item, nil
}

func encodeCurrentCustomerCursor(cursor currentCustomerCursor) (string, error) {
	if cursor.ID < 1 || cursor.SubmittedAt.IsZero() {
		return "", gerror.New("customer tax invoice cursor is invalid")
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCurrentCustomerCursor(value string) (*currentCustomerCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > 256 {
		return nil, gerror.New("customer tax invoice cursor is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, gerror.New("customer tax invoice cursor is invalid")
	}
	var cursor currentCustomerCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.ID < 1 || cursor.SubmittedAt.IsZero() {
		return nil, gerror.New("customer tax invoice cursor is invalid")
	}
	cursor.SubmittedAt = cursor.SubmittedAt.UTC()
	return &cursor, nil
}
