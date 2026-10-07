package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskReviewRejection(ctx context.Context, req *v1.ProductFulfillmentTaskReviewRejectionReq) (*v1.ProductFulfillmentTaskReviewRejectionRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.RejectToManual(ctx, productFulfillmentActionInput(req.ProductFulfillmentActionReq))
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentTaskReviewRejectionRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
