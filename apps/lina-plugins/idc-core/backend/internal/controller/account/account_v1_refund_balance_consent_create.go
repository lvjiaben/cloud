// This file handles current-customer mixed-source balance refund decisions.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) RefundBalanceConsentCreate(
	ctx context.Context,
	req *v1.RefundBalanceConsentCreateReq,
) (*v1.RefundBalanceConsentCreateRes, error) {
	refund, err := c.refunds.RespondRefundBalanceConsent(ctx, billing.RefundBalanceConsentInput{
		RefundNo: req.RefundNo, ExpectedVersion: req.ExpectedVersion,
		Consent: req.Consent, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RefundBalanceConsentCreateRes{Refund: toAPIRefund(refund)}, nil
}
