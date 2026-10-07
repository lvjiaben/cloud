package contentadmin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/contenthub"
)

type ControllerV1 struct{ service contenthub.Service }

func NewV1(service contenthub.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("content admin controller requires service")
	}
	return &ControllerV1{service: service}, nil
}
func (c *ControllerV1) ContentList(ctx context.Context, req *v1.ContentListReq) (*v1.ContentListRes, error) {
	page, err := c.service.SearchAdmin(ctx, contenthub.SearchInput{Type: req.Type, CategoryNo: req.CategoryNo, Status: req.Status, Visibility: req.Visibility, Query: req.Query, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ContentItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, ToItem(item))
	}
	return &v1.ContentListRes{Items: items, NextCursor: page.NextCursor}, nil
}
func (c *ControllerV1) ContentUploadPolicy(ctx context.Context, _ *v1.ContentUploadPolicyReq) (*v1.ContentUploadPolicyRes, error) {
	policy, err := c.service.GetUploadPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ContentUploadPolicyRes{
		OrdinaryMaxSizeBytes:           policy.OrdinaryMaxSizeBytes,
		InspectionRequiredMaxSizeBytes: policy.InspectionRequiredMaxSizeBytes,
		InspectionRequiredAvailable:    policy.InspectionRequiredAvailable,
		OrdinaryExtensions:             policy.OrdinaryExtensions,
		InspectionRequiredExtensions:   policy.InspectionRequiredExtensions,
	}, nil
}
func (c *ControllerV1) ContentCategoryList(ctx context.Context, req *v1.ContentCategoryListReq) (*v1.ContentCategoryListRes, error) {
	page, err := c.service.SearchCategories(ctx, contenthub.CategorySearchInput{Type: req.Type, Status: req.Status, Locale: req.Locale, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]v1.ContentCategory, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, *ToCategory(&item))
	}
	return &v1.ContentCategoryListRes{Items: items, NextCursor: page.NextCursor}, nil
}
func (c *ControllerV1) ContentCategoryCreate(ctx context.Context, req *v1.ContentCategoryCreateReq) (*v1.ContentCategoryCreateRes, error) {
	item, err := c.service.CreateCategory(ctx, contenthub.CategoryCreateInput{Type: req.Type, Name: req.Name, SortOrder: req.SortOrder})
	if err != nil {
		return nil, err
	}
	return &v1.ContentCategoryCreateRes{Item: ToCategory(item)}, nil
}
func (c *ControllerV1) ContentCategoryDelete(ctx context.Context, req *v1.ContentCategoryDeleteReq) (*v1.ContentCategoryDeleteRes, error) {
	if err := c.service.DeleteCategory(ctx, contenthub.CategoryDeleteInput{CategoryNo: req.CategoryNo, Version: req.Version, Confirmed: req.Confirmed}); err != nil {
		return nil, err
	}
	return &v1.ContentCategoryDeleteRes{}, nil
}
func (c *ControllerV1) ContentDetail(ctx context.Context, req *v1.ContentDetailReq) (*v1.ContentDetailRes, error) {
	item, err := c.service.DetailAdmin(ctx, req.ContentNo)
	if err != nil {
		return nil, err
	}
	return &v1.ContentDetailRes{Item: ToItem(item)}, nil
}
func writeInput(input v1.ContentWriteFields) contenthub.WriteInput {
	return contenthub.WriteInput{ContentNo: input.ContentNo, Type: input.Type, CategoryNo: input.CategoryNo, Visibility: input.Visibility, Title: input.Title, Summary: input.Summary, Body: input.Body, SEOTitle: input.SEOTitle, SEODescription: input.SEODescription, Status: input.Status, Version: input.Version, TargetCustomerNos: input.TargetCustomerNos, TargetServiceNos: input.TargetServiceNos, TargetCustomerGroupNos: input.TargetCustomerGroupNos}
}
func (c *ControllerV1) ContentCreate(ctx context.Context, req *v1.ContentCreateReq) (*v1.ContentCreateRes, error) {
	item, err := c.service.Create(ctx, writeInput(req.ContentWriteFields))
	if err != nil {
		return nil, err
	}
	return &v1.ContentCreateRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) ContentUpdate(ctx context.Context, req *v1.ContentUpdateReq) (*v1.ContentUpdateRes, error) {
	item, err := c.service.Update(ctx, writeInput(req.ContentWriteFields))
	if err != nil {
		return nil, err
	}
	return &v1.ContentUpdateRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) ContentPublish(ctx context.Context, req *v1.ContentPublishReq) (*v1.ContentPublishRes, error) {
	item, err := c.service.Publish(ctx, req.ContentNo, req.Version)
	if err != nil {
		return nil, err
	}
	return &v1.ContentPublishRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) ContentUnpublish(ctx context.Context, req *v1.ContentUnpublishReq) (*v1.ContentUnpublishRes, error) {
	item, err := c.service.Unpublish(ctx, req.ContentNo, req.Version)
	if err != nil {
		return nil, err
	}
	return &v1.ContentUnpublishRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) ContentRepublish(ctx context.Context, req *v1.ContentRepublishReq) (*v1.ContentRepublishRes, error) {
	item, err := c.service.Republish(ctx, req.ContentNo, req.Version)
	if err != nil {
		return nil, err
	}
	return &v1.ContentRepublishRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) ContentDelete(ctx context.Context, req *v1.ContentDeleteReq) (*v1.ContentDeleteRes, error) {
	if !req.Confirmed {
		return nil, bizerr.NewCode(contenthub.CodeInvalid)
	}
	item, err := c.service.Delete(ctx, req.ContentNo, req.Version)
	if err != nil {
		return nil, err
	}
	return &v1.ContentDeleteRes{Item: ToItem(item)}, nil
}
func audienceInput(req v1.AudienceSearchReq) contenthub.AudienceSearchInput {
	return contenthub.AudienceSearchInput{Keyword: req.Keyword, Cursor: req.Cursor, Limit: req.Limit}
}
func (c *ControllerV1) AudienceCustomerSearch(ctx context.Context, req *v1.AudienceCustomerSearchReq) (*v1.AudienceCustomerSearchRes, error) {
	page, err := c.service.SearchAudienceCustomers(ctx, audienceInput(req.AudienceSearchReq))
	if err != nil {
		return nil, err
	}
	items := make([]v1.AudienceCandidate, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.AudienceCandidate{CustomerNo: item.Number, Label: item.Label})
	}
	return (*v1.AudienceCustomerSearchRes)(&v1.AudienceSearchRes{Items: items, NextCursor: page.NextCursor}), nil
}
func (c *ControllerV1) AudienceServiceSearch(ctx context.Context, req *v1.AudienceServiceSearchReq) (*v1.AudienceServiceSearchRes, error) {
	page, err := c.service.SearchAudienceServices(ctx, audienceInput(req.AudienceSearchReq))
	if err != nil {
		return nil, err
	}
	items := make([]v1.AudienceCandidate, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.AudienceCandidate{ServiceNo: item.Number, Label: item.Label})
	}
	return (*v1.AudienceServiceSearchRes)(&v1.AudienceSearchRes{Items: items, NextCursor: page.NextCursor}), nil
}
func (c *ControllerV1) AudienceCustomerGroupSearch(ctx context.Context, req *v1.AudienceCustomerGroupSearchReq) (*v1.AudienceCustomerGroupSearchRes, error) {
	page, err := c.service.SearchAudienceCustomerGroups(ctx, audienceInput(req.AudienceSearchReq))
	if err != nil {
		return nil, err
	}
	items := make([]v1.AudienceCandidate, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.AudienceCandidate{GroupNo: item.Number, Label: item.Label})
	}
	return (*v1.AudienceCustomerGroupSearchRes)(&v1.AudienceSearchRes{Items: items, NextCursor: page.NextCursor}), nil
}
func (c *ControllerV1) UploadCreate(ctx context.Context, req *v1.UploadCreateReq) (*v1.UploadCreateRes, error) {
	item, err := c.service.CreateUpload(ctx, contenthub.UploadCreateInput{ContentNo: req.ContentNo, FileName: req.FileName, SizeBytes: req.SizeBytes, ContentVersion: req.ContentVersion})
	if err != nil {
		return nil, err
	}
	return &v1.UploadCreateRes{Item: ToUpload(item)}, nil
}
func (c *ControllerV1) UploadDetail(ctx context.Context, req *v1.UploadDetailReq) (*v1.UploadDetailRes, error) {
	item, err := c.service.GetUpload(ctx, req.UploadNo)
	if err != nil {
		return nil, err
	}
	return &v1.UploadDetailRes{Item: ToUpload(item)}, nil
}
func (c *ControllerV1) UploadChunk(ctx context.Context, req *v1.UploadChunkReq) (*v1.UploadChunkRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(contenthub.CodeInvalid)
	}
	item, err := c.service.UploadChunk(ctx, req.UploadNo, req.Offset, request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.UploadChunkRes{UploadNo: item.UploadNo, ConfirmedBytes: item.ConfirmedBytes, NextOffset: item.NextOffset, Version: item.Version, Complete: item.Complete}, nil
}
func (c *ControllerV1) UploadComplete(ctx context.Context, req *v1.UploadCompleteReq) (*v1.UploadCompleteRes, error) {
	item, err := c.service.CompleteUpload(ctx, contenthub.CompleteInput{UploadNo: req.UploadNo, ExpectedSizeBytes: req.ExpectedSizeBytes, Version: req.Version})
	if err != nil {
		return nil, err
	}
	return &v1.UploadCompleteRes{Item: ToCandidate(item)}, nil
}
func (c *ControllerV1) UploadCancel(ctx context.Context, req *v1.UploadCancelReq) (*v1.UploadCancelRes, error) {
	if err := c.service.CancelUpload(ctx, req.UploadNo, req.Version, req.Confirmed); err != nil {
		return nil, err
	}
	return &v1.UploadCancelRes{}, nil
}
func candidateInput(input v1.CandidateActionFields) contenthub.CandidateActionInput {
	return contenthub.CandidateActionInput{ContentNo: input.ContentNo, FileNo: input.FileNo, ContentVersion: input.ContentVersion, FileVersion: input.FileVersion, Confirmed: input.Confirmed}
}
func (c *ControllerV1) CandidateCancel(ctx context.Context, req *v1.CandidateCancelReq) (*v1.CandidateCancelRes, error) {
	item, err := c.service.CancelCandidate(ctx, candidateInput(req.CandidateActionFields))
	if err != nil {
		return nil, err
	}
	return &v1.CandidateCancelRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) CandidateScanRetry(ctx context.Context, req *v1.CandidateScanRetryReq) (*v1.CandidateScanRetryRes, error) {
	item, err := c.service.RetryCandidateScan(ctx, candidateInput(req.CandidateActionFields))
	if err != nil {
		return nil, err
	}
	return &v1.CandidateScanRetryRes{Item: ToItem(item)}, nil
}
func (c *ControllerV1) CandidateEnable(ctx context.Context, req *v1.CandidateEnableReq) (*v1.CandidateEnableRes, error) {
	item, err := c.service.EnableCandidate(ctx, candidateInput(req.CandidateActionFields))
	if err != nil {
		return nil, err
	}
	return &v1.CandidateEnableRes{Item: ToItem(item)}, nil
}

func ToItem(item *contenthub.Item) *v1.ContentItem {
	if item == nil {
		return nil
	}
	result := &v1.ContentItem{ContentNo: item.ContentNo, Type: item.Type, CategoryNo: item.CategoryNo, Visibility: item.Visibility, Title: item.Title, Summary: item.Summary, Body: item.Body, SEOTitle: item.SEOTitle, SEODescription: item.SEODescription, PublicPath: item.PublicPath, Status: item.Status, CurrentFileNo: item.CurrentFileNo, ValidityVersion: item.ValidityVersion, Version: item.Version, TargetCustomerNos: item.TargetCustomerNos, TargetServiceNos: item.TargetServiceNos, TargetCustomerGroupNos: item.TargetCustomerGroupNos, ActiveUpload: ToActiveUpload(item.ActiveUpload), CandidateFile: ToCandidate(item.CandidateFile), AvailableActions: item.AvailableActions}
	if item.CurrentFile != nil {
		result.CurrentFile = &v1.CurrentContentFile{FileNo: item.CurrentFile.FileNo, FileName: item.CurrentFile.FileName, MediaType: item.CurrentFile.MediaType, ContentKind: item.CurrentFile.ContentKind, SizeBytes: item.CurrentFile.SizeBytes, SHA256: item.CurrentFile.SHA256, Version: item.CurrentFile.Version}
	}
	result.TargetAudiences = make([]v1.TargetAudience, 0, len(item.TargetAudiences))
	for _, audience := range item.TargetAudiences {
		result.TargetAudiences = append(result.TargetAudiences, v1.TargetAudience{Kind: audience.Kind, Number: audience.Number, Label: audience.Label})
	}
	if item.PublishedAt != nil {
		result.PublishedAt = item.PublishedAt.UnixMilli()
	}
	if item.OfflineAt != nil {
		result.OfflineAt = item.OfflineAt.UnixMilli()
	}
	if item.CreatedAt != nil {
		result.CreatedAt = item.CreatedAt.UnixMilli()
	}
	if item.UpdatedAt != nil {
		result.UpdatedAt = item.UpdatedAt.UnixMilli()
	}
	return result
}
func ToCategory(item *contenthub.Category) *v1.ContentCategory {
	if item == nil {
		return nil
	}
	result := &v1.ContentCategory{CategoryNo: item.CategoryNo, Type: item.Type, Name: item.Name, SortOrder: item.SortOrder, Status: item.Status, Version: item.Version, Synthetic: item.Synthetic}
	if !item.CreatedAt.IsZero() {
		result.CreatedAt = item.CreatedAt.UnixMilli()
	}
	if !item.UpdatedAt.IsZero() {
		result.UpdatedAt = item.UpdatedAt.UnixMilli()
	}
	return result
}
func ToActiveUpload(item *contenthub.ActiveUpload) *v1.ActiveUpload {
	if item == nil {
		return nil
	}
	return &v1.ActiveUpload{UploadNo: item.UploadNo, FileName: item.FileName, SizeBytes: item.SizeBytes, ConfirmedBytes: item.ConfirmedBytes, ChunkSizeBytes: item.ChunkSizeBytes, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version, CanResume: item.CanResume, CanCancel: item.CanCancel}
}
func ToCandidate(item *contenthub.CandidateFile) *v1.CandidateFile {
	if item == nil {
		return nil
	}
	result := &v1.CandidateFile{FileNo: item.FileNo, FileName: item.FileName, MediaType: item.MediaType, ContentKind: item.ContentKind, GateStatus: item.GateStatus, ScanStatus: item.ScanStatus, SizeBytes: item.SizeBytes, Version: item.Version, CanCancel: item.CanCancel, CanRescan: item.CanRescan, CanEnable: item.CanEnable}
	if item.GateExpiresAt != nil {
		result.GateExpiresAt = item.GateExpiresAt.UnixMilli()
	}
	return result
}
func ToUpload(item *contenthub.UploadSession) *v1.UploadSession {
	if item == nil {
		return nil
	}
	return &v1.UploadSession{UploadNo: item.UploadNo, FileName: item.FileName, SizeBytes: item.SizeBytes, ConfirmedBytes: item.ConfirmedBytes, ChunkSizeBytes: item.ChunkSizeBytes, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}
}
