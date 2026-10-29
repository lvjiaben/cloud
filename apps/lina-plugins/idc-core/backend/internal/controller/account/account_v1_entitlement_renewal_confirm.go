// This file confirms an owned renewal quotation; invoice payment remains in billing.
package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

func (c *ControllerV1) EntitlementRenewalConfirm(ctx context.Context, req *v1.EntitlementRenewalConfirmReq) (*v1.EntitlementRenewalConfirmRes, error) {
	operation, err := c.renewals.ConfirmCurrentCustomer(ctx, renewal.ConfirmInput{QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.EntitlementRenewalConfirmRes{Operation: toAPITrialConversionOperation(operation)}, nil
}
