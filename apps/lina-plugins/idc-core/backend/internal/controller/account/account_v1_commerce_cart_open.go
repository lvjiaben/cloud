package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommerceCartOpen(ctx context.Context, req *v1.CommerceCartOpenReq) (res *v1.CommerceCartOpenRes, err error) {
	cart, err := c.commerce.OpenAccountCart(ctx, req.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceCartOpenRes{Cart: toAPICommerceCart(cart)}, nil
}
