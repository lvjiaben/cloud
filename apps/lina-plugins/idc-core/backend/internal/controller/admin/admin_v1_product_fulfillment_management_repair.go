// This file delegates local-only management repair to the authorized owner.
package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentManagementRepair(ctx context.Context, req *v1.ProductFulfillmentManagementRepairReq) (*v1.ProductFulfillmentManagementRepairRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.RepairManagement(ctx, productfulfillment.ManagementRepairInput{IntentNo: req.IntentNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentManagementRepairRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
