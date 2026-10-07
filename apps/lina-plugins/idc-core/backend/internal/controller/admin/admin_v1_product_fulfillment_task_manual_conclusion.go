package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskManualConclusion(ctx context.Context, req *v1.ProductFulfillmentTaskManualConclusionReq) (*v1.ProductFulfillmentTaskManualConclusionRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.ConcludeManual(ctx, productfulfillment.ManualConclusionInput{
		ActionInput: productfulfillment.ActionInput{
			IntentNo: req.IntentNo, ExpectedVersion: req.ExpectedVersion,
			IdempotencyKey: req.IdempotencyKey, ReasonCode: req.ReasonCode, Note: req.Note,
		},
		Outcome: req.OutcomeCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentTaskManualConclusionRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}

func productFulfillmentActionInput(input v1.ProductFulfillmentActionReq) productfulfillment.ActionInput {
	return productfulfillment.ActionInput{
		IntentNo: input.IntentNo, ExpectedVersion: input.ExpectedVersion,
		IdempotencyKey: input.IdempotencyKey, ReasonCode: input.ReasonCode, Note: input.Note,
	}
}
