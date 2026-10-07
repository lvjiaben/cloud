package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskDetail(ctx context.Context, req *v1.ProductFulfillmentTaskDetailReq) (*v1.ProductFulfillmentTaskDetailRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.Detail(ctx, req.IntentNo)
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentTaskDetailRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
