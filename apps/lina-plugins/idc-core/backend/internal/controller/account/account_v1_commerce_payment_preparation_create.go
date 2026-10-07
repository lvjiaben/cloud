package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommercePaymentPreparationCreate(ctx context.Context, req *v1.CommercePaymentPreparationCreateReq) (res *v1.CommercePaymentPreparationCreateRes, err error) {
	preparation, err := c.paymentPreparation.PrepareAccountOrderPayment(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return &v1.CommercePaymentPreparationCreateRes{Preparation: toAPICommercePaymentPreparation(preparation)}, nil
}
