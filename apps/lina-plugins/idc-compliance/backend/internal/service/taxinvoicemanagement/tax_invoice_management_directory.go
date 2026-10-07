package taxinvoicemanagement

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"lina-plugin-idc-compliance/backend/internal/service/numberedpage"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const directoryTarget = "tax-invoice-directory"

type directoryCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int64     `json:"id"`
}

func (s *serviceImpl) List(ctx context.Context, query ListQuery) (*Page, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorize(ctx, PermissionQuery, directoryTarget); err != nil {
		return nil, err
	}
	query.CustomerNo, query.StatusCode, query.Cursor = strings.TrimSpace(query.CustomerNo), strings.TrimSpace(query.StatusCode), strings.TrimSpace(query.Cursor)
	if query.Limit == 0 {
		query.Limit = 20
	}
	if !numberedpage.Valid(query.Page, query.Limit, query.Cursor) || query.Limit < 1 || query.Limit > 100 || len(query.CustomerNo) > 64 || len(query.StatusCode) > 32 || len(query.Cursor) > 256 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var cursor *directoryCursor
	if query.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		var value directoryCursor
		if json.Unmarshal(decoded, &value) != nil || value.ID <= 0 || value.CreatedAt.IsZero() {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		cursor = &value
	}
	model := s.database.Model("plugin_idc_compliance_tax_invoice").Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).WhereNotNull("submitted_at")
	if query.CustomerNo != "" {
		model = model.Where("customer_no", query.CustomerNo)
	}
	if query.StatusCode != "" {
		model = model.Where("status_code", query.StatusCode)
	}
	countModel := model.Clone()
	if cursor != nil {
		model = model.Where("(submitted_at < ?) OR (submitted_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []*requestRow
	total, err := numberedpage.Scan(ctx, s.database, countModel, model.Fields("id, tax_invoice_no, customer_no, status_code, invoice_type_code, provider_code, external_invoice_ref, decision_reason_code, decision_internal_note, gross_amount_minor, net_amount_minor, tax_amount_minor, tax_currency_code, masked_invoice_number, version, submitted_at, reviewed_at, issued_at, delivered_at, canceled_at, provider_not_issued_confirmed_at").OrderDesc("submitted_at").OrderDesc("id").Offset(max(query.Page-1, 0)*query.Limit).Limit(query.Limit+1), &rows)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}
	capability := s.currentCapability(ctx)
	page := &Page{Total: total, Items: make([]Request, 0, len(rows))}
	for _, row := range rows {
		item, projectErr := project(row, capability)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Items = append(page.Items, *item)
	}
	if hasMore && len(rows) > 0 {
		raw, err := json.Marshal(directoryCursor{CreatedAt: rows[len(rows)-1].SubmittedAt.UTC(), ID: rows[len(rows)-1].ID})
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}

func (s *serviceImpl) GetDetail(ctx context.Context, taxInvoiceNo string) (*Detail, error) {
	request, err := s.Get(ctx, taxInvoiceNo)
	if err != nil {
		return nil, err
	}
	detail := &Detail{Request: *request}
	detailBuyer, err := s.loadBuyerProfile(ctx, taxInvoiceNo)
	if err != nil {
		return nil, err
	}
	detail.Buyer = *detailBuyer
	if err = s.database.GetScan(ctx, &detail.Allocations, `SELECT allocation_no, billing_invoice_no, order_no, allocated_gross_minor AS gross_amount_minor, allocated_net_minor AS net_amount_minor, allocated_tax_minor AS tax_amount_minor, refunded_gross_minor, red_lettered_gross_minor FROM plugin_idc_compliance_tax_invoice_allocation WHERE tenant_id = ? AND tax_invoice_no = ? ORDER BY id LIMIT 101`, idccap.DefaultTenantID, taxInvoiceNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(detail.Allocations) > 100 {
		detail.Allocations = detail.Allocations[:100]
		detail.HistoryTruncated = true
	}
	if err = s.database.GetScan(ctx, &detail.Items, `SELECT item_no, allocation_no, billing_invoice_item_no, line_no, tax_classification_code, tax_rate_ppm, gross_amount_minor, net_amount_minor, tax_amount_minor FROM plugin_idc_compliance_tax_invoice_item WHERE tenant_id = ? AND tax_invoice_no = ? ORDER BY line_no LIMIT 201`, idccap.DefaultTenantID, taxInvoiceNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(detail.Items) > 200 {
		detail.Items = detail.Items[:200]
		detail.HistoryTruncated = true
	}
	if err = s.database.GetScan(ctx, &detail.Files, `SELECT file_no, file_type_code, content_sha256, media_type, byte_size, status_code, immutable, archived_at, created_at FROM plugin_idc_compliance_tax_invoice_file WHERE tenant_id = ? AND tax_invoice_no = ? ORDER BY created_at DESC, id DESC LIMIT 51`, idccap.DefaultTenantID, taxInvoiceNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(detail.Files) > 50 {
		detail.Files = detail.Files[:50]
		detail.HistoryTruncated = true
	}
	return detail, nil
}
