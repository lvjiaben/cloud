package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskTakeover(ctx context.Context, req *v1.ProductFulfillmentTaskTakeoverReq) (*v1.ProductFulfillmentTaskTakeoverRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.Takeover(ctx, productFulfillmentActionInput(req.ProductFulfillmentActionReq))
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentTaskTakeoverRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
