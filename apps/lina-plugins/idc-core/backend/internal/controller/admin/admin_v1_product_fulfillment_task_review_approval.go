package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskReviewApproval(ctx context.Context, req *v1.ProductFulfillmentTaskReviewApprovalReq) (*v1.ProductFulfillmentTaskReviewApprovalRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.ApproveAndStart(ctx, productFulfillmentActionInput(req.ProductFulfillmentActionReq))
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentTaskReviewApprovalRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
