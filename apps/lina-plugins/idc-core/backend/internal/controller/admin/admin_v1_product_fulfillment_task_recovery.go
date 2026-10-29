// This file delegates recovery to the authorized fulfillment owner command.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentTaskRecovery(ctx context.Context, req *v1.ProductFulfillmentTaskRecoveryReq) (res *v1.ProductFulfillmentTaskRecoveryRes, err error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.RecoverPreparation(ctx, productfulfillment.ActionInput{
		IntentNo: req.IntentNo, ExpectedVersion: req.ExpectedVersion, RecoveryConfiguration: req.RecoveryConfiguration,
		IdempotencyKey: req.IdempotencyKey, ReasonCode: req.ReasonCode, Note: req.Note,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentTaskRecoveryRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
