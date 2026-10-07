// This file maps the isolated upstream-ticket service to admin HTTP DTOs.
package supportadmin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func (c *ControllerV1) SupportUpstreamChannelGet(ctx context.Context, _ *v1.SupportUpstreamChannelGetReq) (*v1.SupportUpstreamChannelGetRes, error) {
	item, err := c.service.UpstreamChannel(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamChannelGetRes{Item: toUpstreamChannel(item)}, nil
}

func (c *ControllerV1) SupportUpstreamChannelPut(ctx context.Context, req *v1.SupportUpstreamChannelPutReq) (*v1.SupportUpstreamChannelPutRes, error) {
	item, err := c.service.ConfigureUpstreamChannel(ctx, supportdesk.UpstreamChannelInput{OperationNo: req.OperationNo, BaseURL: req.BaseURL, AuthMode: req.AuthMode, Username: req.Username, Password: req.Password, Status: req.Status, Reason: req.Reason, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamChannelPutRes{Item: toUpstreamChannel(item)}, nil
}

func (c *ControllerV1) SupportUpstreamDepartmentMapPut(ctx context.Context, req *v1.SupportUpstreamDepartmentMapPutReq) (*v1.SupportUpstreamDepartmentMapPutRes, error) {
	item, err := c.service.SaveUpstreamDepartmentMap(ctx, supportdesk.UpstreamDepartmentMapInput{DepartmentNo: req.DepartmentNo, UpstreamDepartmentID: req.UpstreamDepartmentID, UpstreamDepartmentName: req.UpstreamDepartmentName, Status: req.Status, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamDepartmentMapPutRes{Item: toUpstreamDepartmentMap(item)}, nil
}

func (c *ControllerV1) SupportUpstreamDepartmentMapGet(ctx context.Context, req *v1.SupportUpstreamDepartmentMapGetReq) (*v1.SupportUpstreamDepartmentMapGetRes, error) {
	item, err := c.service.UpstreamDepartmentMap(ctx, req.DepartmentNo)
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamDepartmentMapGetRes{Item: toUpstreamDepartmentMap(item)}, nil
}

func (c *ControllerV1) SupportUpstreamEntitlementMapPut(ctx context.Context, req *v1.SupportUpstreamEntitlementMapPutReq) (*v1.SupportUpstreamEntitlementMapPutRes, error) {
	item, err := c.service.SaveUpstreamEntitlementMap(ctx, supportdesk.UpstreamEntitlementMapInput{EntitlementNo: req.EntitlementNo, UpstreamReference: req.UpstreamReference, Status: req.Status, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamEntitlementMapPutRes{Item: toUpstreamEntitlementMap(item)}, nil
}

func (c *ControllerV1) SupportUpstreamEntitlementMapGet(ctx context.Context, req *v1.SupportUpstreamEntitlementMapGetReq) (*v1.SupportUpstreamEntitlementMapGetRes, error) {
	item, err := c.service.UpstreamEntitlementMap(ctx, req.EntitlementNo)
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamEntitlementMapGetRes{Item: toUpstreamEntitlementMap(item)}, nil
}

func (c *ControllerV1) SupportUpstreamPrepare(ctx context.Context, req *v1.SupportUpstreamPrepareReq) (*v1.SupportUpstreamPrepareRes, error) {
	item, err := c.service.PrepareUpstream(ctx, supportdesk.UpstreamPrepareInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion, OperationNo: req.OperationNo, MessageNos: req.MessageNos, AttachmentNos: req.AttachmentNos})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamPrepareRes{Item: toUpstreamPreview(item)}, nil
}

func (c *ControllerV1) SupportUpstreamConfirm(ctx context.Context, req *v1.SupportUpstreamConfirmReq) (*v1.SupportUpstreamConfirmRes, error) {
	item, err := c.service.ConfirmUpstream(ctx, supportdesk.UpstreamConfirmInput{ForwardNo: req.ForwardNo, PreviewDigest: req.PreviewDigest, OperationNo: req.OperationNo, Reason: req.Reason, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamConfirmRes{Item: toUpstreamForward(item)}, nil
}

func (c *ControllerV1) SupportUpstreamRetry(ctx context.Context, req *v1.SupportUpstreamRetryReq) (*v1.SupportUpstreamRetryRes, error) {
	item, err := c.service.RetryUpstream(ctx, supportdesk.UpstreamConfirmInput{ForwardNo: req.ForwardNo, PreviewDigest: req.PreviewDigest, OperationNo: req.OperationNo, Reason: req.Reason, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamRetryRes{Item: toUpstreamForward(item)}, nil
}

func (c *ControllerV1) SupportUpstreamForwardGet(ctx context.Context, req *v1.SupportUpstreamForwardGetReq) (*v1.SupportUpstreamForwardGetRes, error) {
	item, err := c.service.UpstreamForward(ctx, req.ForwardNo)
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamForwardGetRes{Item: toUpstreamForward(item)}, nil
}

func (c *ControllerV1) SupportTicketUpstreamForwardList(ctx context.Context, req *v1.SupportTicketUpstreamForwardListReq) (*v1.SupportTicketUpstreamForwardListRes, error) {
	items, err := c.service.UpstreamForwards(ctx, req.TicketNo)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.SupportUpstreamForward, 0, len(items))
	for index := range items {
		result = append(result, toUpstreamForward(&items[index]))
	}
	return &v1.SupportTicketUpstreamForwardListRes{Items: result}, nil
}

func (c *ControllerV1) SupportUpstreamSync(ctx context.Context, req *v1.SupportUpstreamSyncReq) (*v1.SupportUpstreamSyncRes, error) {
	added, err := c.service.SyncUpstreamReplies(ctx, req.ForwardNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamSyncRes{Added: added}, nil
}

func (c *ControllerV1) SupportUpstreamPublish(ctx context.Context, req *v1.SupportUpstreamPublishReq) (*v1.SupportUpstreamPublishRes, error) {
	item, err := c.service.PublishUpstreamReply(ctx, supportdesk.UpstreamPublishInput{TicketNo: req.TicketNo, LocalMessageNo: req.LocalMessageNo, Reason: req.Reason, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SupportUpstreamPublishRes{Item: ToDetail(item)}, nil
}

func toUpstreamChannel(item *supportdesk.UpstreamChannel) *v1.SupportUpstreamChannel {
	if item == nil {
		return nil
	}
	updatedAt := int64(0)
	if !item.UpdatedAt.IsZero() {
		updatedAt = item.UpdatedAt.UnixMilli()
	}
	return &v1.SupportUpstreamChannel{BaseURL: item.BaseURL, AuthMode: item.AuthMode, Status: item.Status, CredentialMaskedTail: item.CredentialMaskedTail, Configured: item.Configured, Version: item.Version, UpdatedAt: updatedAt}
}

func toUpstreamDepartmentMap(item *supportdesk.UpstreamDepartmentMap) *v1.SupportUpstreamDepartmentMap {
	if item == nil {
		return nil
	}
	return &v1.SupportUpstreamDepartmentMap{DepartmentNo: item.DepartmentNo, UpstreamDepartmentID: item.UpstreamDepartmentID, UpstreamDepartmentName: item.UpstreamDepartmentName, Status: item.Status, Version: item.Version}
}

func toUpstreamEntitlementMap(item *supportdesk.UpstreamEntitlementMap) *v1.SupportUpstreamEntitlementMap {
	if item == nil {
		return nil
	}
	return &v1.SupportUpstreamEntitlementMap{EntitlementNo: item.EntitlementNo, UpstreamReference: item.UpstreamReference, Status: item.Status, Version: item.Version}
}

func toUpstreamPreview(item *supportdesk.UpstreamPreview) *v1.SupportUpstreamPreview {
	if item == nil {
		return nil
	}
	return &v1.SupportUpstreamPreview{ForwardNo: item.ForwardNo, TicketNo: item.TicketNo, Title: item.Title, Content: item.Content, SelectionDigest: item.SelectionDigest, PreviewDigest: item.PreviewDigest, MessageNos: item.MessageNos, AttachmentNos: item.AttachmentNos, DepartmentID: item.DepartmentID, EntitlementUpstreamRef: item.EntitlementUpstreamRef, Version: item.Version}
}

func toUpstreamForward(item *supportdesk.UpstreamForward) *v1.SupportUpstreamForward {
	if item == nil {
		return nil
	}
	updatedAt := int64(0)
	if !item.UpdatedAt.IsZero() {
		updatedAt = item.UpdatedAt.UnixMilli()
	}
	return &v1.SupportUpstreamForward{ForwardNo: item.ForwardNo, TicketNo: item.TicketNo, ProviderTicketReference: item.ProviderTicketReference, Status: item.Status, ResultClassification: item.ResultClassification, EvidenceCode: item.EvidenceCode, PreviewDigest: item.PreviewDigest, RetryEligible: item.RetryEligible, AttemptCount: item.AttemptCount, Version: item.Version, UpdatedAt: updatedAt}
}
