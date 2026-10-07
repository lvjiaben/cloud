package admin

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/financeops"
)

func (c *ControllerV1) FinancialInvoiceDeliveryPreview(ctx context.Context, req *v1.FinancialInvoiceDeliveryPreviewReq) (*v1.FinancialInvoiceDeliveryPreviewRes, error) {
	value, err := c.financialOperations.PreviewInvoiceDelivery(ctx, req.InvoiceNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceDeliveryPreviewRes{Preview: &v1.FinancialInvoiceDeliveryPreview{InvoiceNo: value.InvoiceNo, MaskedEmail: value.MaskedEmail, InvoiceVersion: value.InvoiceVersion}}, nil
}
func (c *ControllerV1) FinancialInvoiceResend(ctx context.Context, req *v1.FinancialInvoiceResendReq) (*v1.FinancialInvoiceResendRes, error) {
	value, err := c.financialOperations.ResendInvoice(ctx, req.InvoiceNo, req.RequestNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceResendRes{IntentNo: value.IntentNo, Reused: value.Reused}, nil
}
func (c *ControllerV1) FinancialInternalNoteList(ctx context.Context, req *v1.FinancialInternalNoteListReq) (*v1.FinancialInternalNoteListRes, error) {
	rows, err := c.financialOperations.ListInternalNotes(ctx, req.TargetType, req.TargetNo, req.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.FinancialInternalNote, 0, len(rows))
	for _, row := range rows {
		result = append(result, noteToAPI(row))
	}
	return &v1.FinancialInternalNoteListRes{Notes: result}, nil
}
func (c *ControllerV1) FinancialInternalNoteCreate(ctx context.Context, req *v1.FinancialInternalNoteCreateReq) (*v1.FinancialInternalNoteCreateRes, error) {
	row, err := c.financialOperations.AddInternalNote(ctx, financeops.AddNoteInput{TargetType: req.TargetType, TargetNo: req.TargetNo, Content: req.Content, CorrectsNoteNo: req.CorrectsNoteNo, IdempotencyKey: req.RequestNo})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInternalNoteCreateRes{Note: noteToAPI(*row)}, nil
}
func noteToAPI(row financeops.InternalNote) *v1.FinancialInternalNote {
	return &v1.FinancialInternalNote{NoteNo: row.NoteNo, TargetType: row.TargetType, TargetNo: row.TargetNo, Content: row.Content, CorrectsNoteNo: row.CorrectsNoteNo, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.UnixMilli()}
}
func (c *ControllerV1) FinancialSavedFilterList(ctx context.Context, req *v1.FinancialSavedFilterListReq) (*v1.FinancialSavedFilterListRes, error) {
	rows, err := c.financialOperations.ListSavedFilters(ctx, req.ViewCode)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.FinancialSavedFilter, 0, len(rows))
	for _, row := range rows {
		result = append(result, filterToAPI(row))
	}
	return &v1.FinancialSavedFilterListRes{Filters: result}, nil
}
func (c *ControllerV1) FinancialSavedFilterSave(ctx context.Context, req *v1.FinancialSavedFilterSaveReq) (*v1.FinancialSavedFilterSaveRes, error) {
	row, err := c.financialOperations.SavePersonalFilter(ctx, financeops.SaveFilterInput{ViewCode: req.ViewCode, DisplayName: req.DisplayName, Filter: req.Filter})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialSavedFilterSaveRes{Filter: filterToAPI(*row)}, nil
}
func filterToAPI(row financeops.SavedFilter) *v1.FinancialSavedFilter {
	return &v1.FinancialSavedFilter{FilterNo: row.FilterNo, ViewCode: row.ViewCode, DisplayName: row.DisplayName, Filter: row.Filter, Version: row.Version, UpdatedAt: row.UpdatedAt.UnixMilli()}
}
func (c *ControllerV1) FinancialExportCreate(ctx context.Context, req *v1.FinancialExportCreateReq) (*v1.FinancialExportCreateRes, error) {
	value, err := c.financialOperations.CreateExport(ctx, financeops.ExportRequestInput{ExportType: req.ExportType, Filter: req.Filter, IdempotencyKey: req.RequestNo})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialExportCreateRes{Export: exportToAPI(value)}, nil
}
func (c *ControllerV1) FinancialExportGenerate(ctx context.Context, req *v1.FinancialExportGenerateReq) (*v1.FinancialExportGenerateRes, error) {
	value, err := c.financialOperations.GenerateExport(ctx, req.RequestNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialExportGenerateRes{Export: exportToAPI(value)}, nil
}
func (c *ControllerV1) FinancialExportStatus(ctx context.Context, req *v1.FinancialExportStatusReq) (*v1.FinancialExportStatusRes, error) {
	value, err := c.financialOperations.GetExportStatus(ctx, req.RequestNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialExportStatusRes{Export: exportToAPI(value)}, nil
}
func (c *ControllerV1) FinancialExportTicket(ctx context.Context, req *v1.FinancialExportTicketReq) (*v1.FinancialExportTicketRes, error) {
	value, err := c.financialOperations.IssueExportTicket(ctx, req.RequestNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialExportTicketRes{Token: value.Token, ExpiresAt: value.ExpiresAt.UnixMilli()}, nil
}
func (c *ControllerV1) FinancialExportOpen(ctx context.Context, req *v1.FinancialExportOpenReq) (res *v1.FinancialExportOpenRes, err error) {
	request := g.RequestFromCtx(ctx)
	file, err := c.financialOperations.OpenExport(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	if request == nil || file == nil || file.Body == nil {
		return nil, gerror.New("financial export stream is unavailable")
	}
	defer func() {
		if closeErr := file.Body.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	request.Response.Header().Set("Content-Type", file.MediaType)
	request.Response.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	safeName := strings.NewReplacer("\\", "_", "\"", "_", "\r", "_", "\n", "_").Replace(file.FileName)
	request.Response.Header().Set("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	if _, err = io.Copy(request.Response.RawWriter(), file.Body); err != nil {
		return nil, err
	}
	request.ExitAll()
	return &v1.FinancialExportOpenRes{}, nil
}
func exportToAPI(value *financeops.ExportStatus) *v1.FinancialExportStatus {
	if value == nil {
		return nil
	}
	result := &v1.FinancialExportStatus{RequestNo: value.RequestNo, ExportType: value.ExportType, Status: value.Status, FailureCode: value.FailureCode, RequestedAt: value.RequestedAt.UnixMilli(), RowCount: value.RowCount, Reused: value.Reused}
	if value.GeneratedAt != nil {
		result.GeneratedAt = value.GeneratedAt.UnixMilli()
	}
	if value.ExpiresAt != nil {
		result.ExpiresAt = value.ExpiresAt.UnixMilli()
	}
	return result
}
