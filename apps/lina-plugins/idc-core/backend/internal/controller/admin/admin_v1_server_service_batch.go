// Package admin projects authorized server operation services onto HTTP contracts.
package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerServiceList(ctx context.Context,
	req *v1.ServerServiceListReq) (*v1.ServerServiceListRes, error) {
	page, err := c.serverBatches.ListServices(ctx, serveroperation.BatchServiceListInput{
		Keyword: req.Keyword, EntitlementStatus: req.EntitlementStatus,
		RuntimeStatus: req.RuntimeStatus, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	res := &v1.ServerServiceListRes{Items: make([]*v1.ServerServiceListItem, 0, len(page.Items)), NextCursor: page.NextCursor, Total: page.Total}
	for _, item := range page.Items {
		projected := &v1.ServerServiceListItem{ServiceNo: item.ServiceNo, CustomerNo: item.CustomerNo,
			ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
			EntitlementStatus: item.EntitlementStatus, RuntimeStatus: item.RuntimeStatus,
			ProviderStatus: item.ProviderStatus}
		if item.EntitlementExpiresAt != nil {
			value := item.EntitlementExpiresAt.UnixMilli()
			projected.EntitlementExpiresAt = &value
		}
		res.Items = append(res.Items, projected)
	}
	return res, nil
}

func (c *ControllerV1) previewServerServiceBatch(ctx context.Context,
	req v1.ServerServiceBatchPreviewFields, dangerous bool) (*v1.ServerServiceBatchPreviewRes, error) {
	if serveroperation.BatchAction(req.Action) == "" {
		return nil, bizerr.NewCode(serveroperation.CodeServerBatchInvalid)
	}
	actualDangerous := serveroperation.BatchAction(req.Action) == serveroperation.BatchActionSuspend ||
		serveroperation.BatchAction(req.Action) == serveroperation.BatchActionReboot ||
		serveroperation.BatchAction(req.Action) == serveroperation.BatchActionReinstall ||
		serveroperation.BatchAction(req.Action) == serveroperation.BatchActionIPChange ||
		serveroperation.BatchAction(req.Action) == serveroperation.BatchActionTerminate
	if actualDangerous != dangerous {
		return nil, bizerr.NewCode(serveroperation.CodeServerBatchInvalid)
	}
	items := make([]serveroperation.BatchPreviewItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		if item == nil {
			continue
		}
		items = append(items, serveroperation.BatchPreviewItemInput{
			ServiceNo: item.ServiceNo, Parameters: item.Parameters,
		})
	}
	batch, err := c.serverBatches.Preview(ctx, serveroperation.BatchPreviewInput{
		Action: serveroperation.BatchAction(req.Action), Items: items, IdempotencyKey: req.RequestNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerServiceBatchPreviewRes{Batch: projectServerServiceBatch(batch)}, nil
}

func (c *ControllerV1) ServerServiceSafeBatchPreview(ctx context.Context,
	req *v1.ServerServiceSafeBatchPreviewReq) (*v1.ServerServiceBatchPreviewRes, error) {
	return c.previewServerServiceBatch(ctx, req.ServerServiceBatchPreviewFields, false)
}

func (c *ControllerV1) ServerServiceDangerousBatchPreview(ctx context.Context,
	req *v1.ServerServiceDangerousBatchPreviewReq) (*v1.ServerServiceBatchPreviewRes, error) {
	return c.previewServerServiceBatch(ctx, req.ServerServiceBatchPreviewFields, true)
}

func (c *ControllerV1) ServerServiceBatchDetail(ctx context.Context,
	req *v1.ServerServiceBatchDetailReq) (*v1.ServerServiceBatchDetailRes, error) {
	batch, err := c.serverBatches.Get(ctx, req.BatchNo)
	if err != nil {
		return nil, err
	}
	return &v1.ServerServiceBatchDetailRes{Batch: projectServerServiceBatch(batch)}, nil
}

func (c *ControllerV1) ServerServiceBatchConfirmation(ctx context.Context,
	req *v1.ServerServiceBatchConfirmationReq) (*v1.ServerServiceBatchConfirmationRes, error) {
	proof, err := c.serverBatches.IssueDangerousConfirmation(ctx, req.BatchNo, req.PreviewDigest)
	if err != nil {
		return nil, err
	}
	return &v1.ServerServiceBatchConfirmationRes{ProofToken: proof.Token,
		ExpiresAt: proof.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) executeServerServiceBatch(ctx context.Context,
	req v1.ServerServiceBatchExecuteFields, dangerous bool) (*v1.ServerServiceBatchExecuteRes, error) {
	batch, err := c.serverBatches.Get(ctx, req.BatchNo)
	if err != nil || batch.Dangerous != dangerous {
		return nil, bizerr.NewCode(serveroperation.CodeServerBatchNotAccessible)
	}
	batch, err = c.serverBatches.Execute(ctx, serveroperation.BatchExecutionInput{
		BatchNo: req.BatchNo, PreviewDigest: req.PreviewDigest, ProofToken: req.ProofToken,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerServiceBatchExecuteRes{Batch: projectServerServiceBatch(batch)}, nil
}

func (c *ControllerV1) ServerServiceSafeBatchExecute(ctx context.Context,
	req *v1.ServerServiceSafeBatchExecuteReq) (*v1.ServerServiceBatchExecuteRes, error) {
	return c.executeServerServiceBatch(ctx, req.ServerServiceBatchExecuteFields, false)
}

func (c *ControllerV1) ServerServiceDangerousBatchExecute(ctx context.Context,
	req *v1.ServerServiceDangerousBatchExecuteReq) (*v1.ServerServiceBatchExecuteRes, error) {
	return c.executeServerServiceBatch(ctx, req.ServerServiceBatchExecuteFields, true)
}

func projectServerServiceBatch(batch *serveroperation.ServiceBatch) *v1.ServerServiceBatch {
	if batch == nil {
		return nil
	}
	result := &v1.ServerServiceBatch{BatchNo: batch.BatchNo, Action: string(batch.Action),
		Dangerous: batch.Dangerous, Status: batch.Status, PreviewDigest: batch.PreviewDigest,
		CreatedAt: batch.CreatedAt.UnixMilli(), Items: make([]*v1.ServerServiceBatchImpactItem, 0, len(batch.Items))}
	for _, item := range batch.Items {
		projected := &v1.ServerServiceBatchImpactItem{LineNumber: item.LineNumber,
			ServiceNo: item.ServiceNo, CustomerNo: item.CustomerNo, ServiceVersion: item.ServiceVersion,
			CurrencyCode: item.CurrencyCode, EntitlementStatus: item.EntitlementStatus,
			RuntimeStatus: item.RuntimeStatus, ProviderStatus: item.ProviderStatus,
			UnpaidAmountMinor: item.UnpaidAmountMinor, ActionAmountMinor: item.ActionAmountMinor,
			ResourceSummary: item.ResourceSummary, Irreversible: item.Irreversible,
			Parameters: item.Parameters, PreviewStatus: item.PreviewStatus, FailureCode: item.FailureCode,
			ExecutionStatus: item.ExecutionStatus, OperationNo: item.OperationNo,
			NotificationEventNo: item.NotificationEventNo}
		if item.EntitlementExpiresAt != nil {
			value := item.EntitlementExpiresAt.UnixMilli()
			projected.EntitlementExpiresAt = &value
		}
		if item.ExecutedAt != nil {
			value := item.ExecutedAt.UnixMilli()
			projected.ExecutedAt = &value
		}
		result.Items = append(result.Items, projected)
	}
	return result
}
