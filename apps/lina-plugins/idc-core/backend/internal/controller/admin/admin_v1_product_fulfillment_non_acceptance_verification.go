// This file delegates non-acceptance verification to the scoped fulfillment owner.
package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func (c *ControllerV1) ProductFulfillmentNonAcceptanceVerification(ctx context.Context, req *v1.ProductFulfillmentNonAcceptanceVerificationReq) (*v1.ProductFulfillmentNonAcceptanceVerificationRes, error) {
	if c.productFulfillment == nil {
		return nil, bizerr.NewCode(productfulfillment.CodeManagementUnavailable)
	}
	detail, err := c.productFulfillment.VerifyNonAcceptance(ctx, productfulfillment.NonAcceptanceInput{ActionInput: productfulfillment.ActionInput{IntentNo: req.IntentNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Note: req.Note}, ConfirmedNoCreation: req.ConfirmedNoCreation})
	if err != nil {
		return nil, err
	}
	return &v1.ProductFulfillmentNonAcceptanceVerificationRes{Task: toAPIProductFulfillmentDetail(detail)}, nil
}
