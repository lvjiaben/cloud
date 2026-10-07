package taxinvoiceredletter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"lina-plugin-idc-compliance/backend/internal/service/numberedpage"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type adminCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int64     `json:"id"`
}

func (s *serviceImpl) ListAdmin(ctx context.Context, query AdminListQuery) (*AdminPage, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.management.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: PermissionQuery, TargetNo: "red-letter-directory"}); err != nil {
		return nil, err
	}
	query.Cursor, query.CustomerNo, query.StatusCode = strings.TrimSpace(query.Cursor), strings.TrimSpace(query.CustomerNo), strings.TrimSpace(query.StatusCode)
	if query.Limit == 0 {
		query.Limit = 20
	}
	if !numberedpage.Valid(query.Page, query.Limit, query.Cursor) || query.Limit < 1 || query.Limit > 100 || len(query.Cursor) > 256 || len(query.CustomerNo) > 64 || len(query.StatusCode) > 24 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var cursor *adminCursor
	if query.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		var value adminCursor
		if json.Unmarshal(decoded, &value) != nil || value.ID <= 0 || value.CreatedAt.IsZero() {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		cursor = &value
	}
	model := s.database.Model("plugin_idc_compliance_red_letter_task").Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID)
	if query.CustomerNo != "" {
		model = model.Where("customer_no", query.CustomerNo)
	}
	if query.StatusCode != "" {
		model = model.Where("status_code", query.StatusCode)
	}
	countModel := model.Clone()
	if cursor != nil {
		model = model.Where("(created_at < ?) OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []*taskRow
	total, err := numberedpage.Scan(ctx, s.database, countModel, model.Fields("id, task_no, tax_invoice_no, refund_operation_no, customer_no, currency_code, status_code, manual_reason_code, allocation_resolved, customer_confirmation_required, provider_code, external_task_ref, gross_amount_minor, net_amount_minor, tax_amount_minor, version, created_at, completed_at, completion_evidence_no").OrderDesc("created_at").OrderDesc("id").Offset(max(query.Page-1, 0)*query.Limit).Limit(query.Limit+1), &rows)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}
	page := &AdminPage{Total: total, Items: make([]Task, 0, len(rows))}
	for _, row := range rows {
		item, err := projectTaskSummary(row)
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		page.Items = append(page.Items, *item)
	}
	if hasMore && len(rows) > 0 {
		raw, err := json.Marshal(adminCursor{CreatedAt: rows[len(rows)-1].CreatedAt.UTC(), ID: rows[len(rows)-1].ID})
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}

func projectTaskSummary(row *taskRow) (*Task, error) {
	if row == nil || row.Version <= 0 || row.GrossAmountMinor <= 0 || row.CreatedAt.IsZero() || row.CustomerNo == nil || row.CurrencyCode == nil {
		return nil, errInconsistent
	}
	item := &Task{TaskNo: row.TaskNo, RefundOperationNo: row.RefundOperationNo, CustomerNo: *row.CustomerNo, CurrencyCode: *row.CurrencyCode, StatusCode: row.StatusCode, ManualReasonCode: row.ManualReasonCode, AllocationResolved: row.AllocationResolved, CustomerConfirmationRequired: row.CustomerConfirmationRequired, GrossAmountMinor: row.GrossAmountMinor, NetAmountMinor: row.NetAmountMinor, TaxAmountMinor: row.TaxAmountMinor, Version: row.Version, CreatedAt: row.CreatedAt.UTC()}
	if row.TaxInvoiceNo != nil {
		item.TaxInvoiceNo = *row.TaxInvoiceNo
	}
	if row.CompletedAt != nil {
		value := row.CompletedAt.UTC()
		item.CompletedAt = &value
	}
	if row.CompletionEvidenceNo != nil {
		item.CompletionEvidenceNo = *row.CompletionEvidenceNo
	}
	return item, nil
}

func (s *serviceImpl) GetAdminDetail(ctx context.Context, taskNo string) (*AdminDetail, error) {
	task, err := s.GetAdmin(ctx, taskNo)
	if err != nil {
		return nil, err
	}
	detail := &AdminDetail{Task: *task}
	var candidateRows []candidateRow
	if err = s.database.GetScan(ctx, &candidateRows, `SELECT refund_allocation_no, tax_invoice_no, tax_invoice_allocation_no, tax_invoice_item_no, requested_gross_minor, available_gross_minor, available_net_minor, available_tax_minor FROM plugin_idc_compliance_red_letter_candidate WHERE tenant_id = ? AND task_no = ? ORDER BY refund_allocation_no, tax_invoice_allocation_no, id LIMIT 201`, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(candidateRows) > 200 {
		candidateRows = candidateRows[:200]
		detail.HistoryTruncated = true
	}
	for _, value := range candidateRows {
		detail.Candidates = append(detail.Candidates, Candidate{RefundAllocationNo: value.RefundAllocationNo, TaxInvoiceNo: value.TaxInvoiceNo, TaxInvoiceAllocationNo: value.TaxInvoiceAllocationNo, TaxInvoiceItemNo: value.TaxInvoiceItemNo, RequestedGrossMinor: value.RequestedGrossMinor, AvailableGrossMinor: value.AvailableGrossMinor, AvailableNetMinor: value.AvailableNetMinor, AvailableTaxMinor: value.AvailableTaxMinor})
	}
	if err = s.database.GetScan(ctx, &detail.Allocations, `SELECT refund_allocation_no, tax_invoice_no, tax_invoice_allocation_no, tax_invoice_item_no, gross_amount_minor, net_amount_minor, tax_amount_minor FROM plugin_idc_compliance_red_letter_allocation WHERE tenant_id = ? AND task_no = ? ORDER BY id LIMIT 201`, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(detail.Allocations) > 200 {
		detail.Allocations = detail.Allocations[:200]
		detail.HistoryTruncated = true
	}
	if task.CompletionEvidenceNo != "" {
		var row struct {
			EvidenceNo, EvidenceReference, EvidenceSHA256, MediaType string
			ByteSize                                                 int64
			OccurredAt                                               time.Time
		}
		if err = s.database.GetScan(ctx, &row, `SELECT evidence_no, source_event_ref AS evidence_reference, payload_sha256 AS evidence_sha256, COALESCE(media_type, '') AS media_type, COALESCE(byte_size, 0) AS byte_size, occurred_at FROM plugin_idc_compliance_external_evidence WHERE tenant_id = ? AND evidence_no = ? AND aggregate_type_code = 'red_letter' AND aggregate_no = ?`, idccap.DefaultTenantID, task.CompletionEvidenceNo, taskNo); err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		row.EvidenceReference = strings.TrimPrefix(row.EvidenceReference, taskNo+":")
		detail.Evidence = &Evidence{EvidenceNo: row.EvidenceNo, EvidenceReference: row.EvidenceReference, EvidenceSHA256: row.EvidenceSHA256, MediaType: row.MediaType, ByteSize: row.ByteSize, OccurredAt: row.OccurredAt.UTC()}
	}
	return detail, nil
}
