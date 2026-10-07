package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommercePaymentPreparationResume(ctx context.Context, req *v1.CommercePaymentPreparationResumeReq) (res *v1.CommercePaymentPreparationResumeRes, err error) {
	preparation, err := c.paymentPreparation.ResumeAccountOrderPayment(ctx, req.ContextNo)
	if err != nil {
		return nil, err
	}
	return &v1.CommercePaymentPreparationResumeRes{Preparation: toAPICommercePaymentPreparation(preparation)}, nil
}
