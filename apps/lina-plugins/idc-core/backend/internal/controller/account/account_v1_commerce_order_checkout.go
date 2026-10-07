package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderCheckout(ctx context.Context, req *v1.CommerceOrderCheckoutReq) (res *v1.CommerceOrderCheckoutRes, err error) {
	checkout, err := c.commerce.CheckoutAccountQuote(ctx, commerce.CheckoutRequest{QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderCheckoutRes{Checkout: toAPICommerceCheckout(checkout)}, nil
}
