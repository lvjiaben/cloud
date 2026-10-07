// This file maps scoped reconciliation pages and the existing two-person review actions.
package admin

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-payment/backend/api/admin/v1"
	"lina-plugin-idc-payment/backend/internal/service/reconciliation"
)

func (c *ControllerV1) ReconciliationHealth(ctx context.Context, req *v1.ReconciliationHealthReq) (*v1.ReconciliationHealthRes, error) {
	page, err := c.reconciliation.ListHealth(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.ReconciliationHealthRes{Channels: make([]*v1.ReconciliationHealthItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		mapped := &v1.ReconciliationHealthItem{ChannelNo: item.ChannelNo, DisplayName: item.DisplayName, ProviderCode: item.ProviderCode, CapabilityCode: item.CapabilityCode, HealthCode: item.HealthCode, LatestBatchNo: item.LatestBatchNo, UnresolvedCount: item.UnresolvedCount, ConsecutiveFailureCount: item.ConsecutiveFailureCount, ResultUnknown: item.ResultUnknown, FailureCode: item.FailureCode}
		if item.LatestBusinessDate != nil {
			value := item.LatestBusinessDate.UnixMilli()
			mapped.LatestBusinessDate = &value
		}
		result.Channels = append(result.Channels, mapped)
	}
	return result, nil
}

func (c *ControllerV1) ReconciliationBatchList(ctx context.Context, req *v1.ReconciliationBatchListReq) (*v1.ReconciliationBatchListRes, error) {
	page, err := c.reconciliation.ListBatches(ctx, req.ChannelNo, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.ReconciliationBatchListRes{Batches: make([]*v1.ReconciliationBatchItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		result.Batches = append(result.Batches, &v1.ReconciliationBatchItem{BatchNo: item.BatchNo, ChannelNo: item.ChannelNo, CapabilityCode: item.CapabilityCode, StatusCode: item.StatusCode, FailureCode: item.FailureCode, BusinessDate: item.BusinessDate.UnixMilli(), TotalCount: item.TotalCount, DifferenceCount: item.DifferenceCount, PaymentCount: item.PaymentCount, RefundCount: item.RefundCount, ResultUnknown: item.ResultUnknown})
	}
	return result, nil
}

func (c *ControllerV1) ReconciliationDifferenceList(ctx context.Context, req *v1.ReconciliationDifferenceListReq) (*v1.ReconciliationDifferenceListRes, error) {
	page, err := c.reconciliation.ListDifferences(ctx, req.BatchNo, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.ReconciliationDifferenceListRes{Differences: make([]*v1.ReconciliationDifferenceItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		result.Differences = append(result.Differences, &v1.ReconciliationDifferenceItem{LineNo: item.LineNo, BatchNo: item.BatchNo, RecordTypeCode: item.RecordType, AttemptNo: item.AttemptNo, RefundAttemptNo: item.RefundAttemptNo, ExternalReference: item.ExternalReference, DifferenceCode: item.DifferenceCode, LocalAmountMinor: strconv.FormatInt(item.LocalAmountMinor, 10), ChannelAmountMinor: strconv.FormatInt(item.ChannelAmountMinor, 10), CurrencyCode: item.CurrencyCode, LocalStatusCode: item.LocalStatusCode, ChannelStatusCode: item.ChannelStatusCode, ResolutionCode: item.ResolutionCode, SupplementRequestNo: item.SupplementRequestNo, SupplementStatusCode: item.SupplementStatusCode, SupplementSubmitterUserID: item.SupplementSubmitterUserID, Version: item.Version, SupplementVersion: item.SupplementVersion})
	}
	return result, nil
}

func (c *ControllerV1) ReconciliationSupplementSubmit(ctx context.Context, req *v1.ReconciliationSupplementSubmitReq) (*v1.ReconciliationSupplementSubmitRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.reconciliation.SubmitSupplement(ctx, req.LineNo, actor, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.ReconciliationSupplementSubmitRes{Supplement: mapSupplement(item)}, nil
}

func (c *ControllerV1) ReconciliationSupplementReview(ctx context.Context, req *v1.ReconciliationSupplementReviewReq) (*v1.ReconciliationSupplementReviewRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.reconciliation.ReviewSupplement(ctx, req.RequestNo, actor, req.DecisionCode == "approve", req.RejectionReasonCode, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.ReconciliationSupplementReviewRes{Supplement: mapSupplement(item)}, nil
}

func (c *ControllerV1) currentActor(ctx context.Context) (int, error) {
	if c.businessContext == nil {
		return 0, gerror.New("payment administration context is unavailable")
	}
	current := c.businessContext.Current(ctx)
	if current.UserID <= 0 || current.IsImpersonation || current.ActingAsTenant {
		return 0, gerror.New("direct authenticated staff is required")
	}
	return current.UserID, nil
}
func mapSupplement(item *reconciliation.Supplement) *v1.ReconciliationSupplementItem {
	if item == nil {
		return nil
	}
	return &v1.ReconciliationSupplementItem{RequestNo: item.RequestNo, LineNo: item.LineNo, AttemptNo: item.AttemptNo, ChannelNo: item.ChannelNo, CurrencyCode: item.CurrencyCode, AmountMinor: strconv.FormatInt(item.AmountMinor, 10), ExternalReference: item.ExternalReference, StatusCode: item.StatusCode, SubmitterUserID: item.SubmitterUserID, ReviewerUserID: item.ReviewerUserID, RejectionReasonCode: item.RejectionReasonCode, Version: item.Version}
}
