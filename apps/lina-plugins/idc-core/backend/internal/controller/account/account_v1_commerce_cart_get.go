package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommerceCartGet(ctx context.Context, _ *v1.CommerceCartGetReq) (res *v1.CommerceCartGetRes, err error) {
	cart, err := c.commerce.GetAccountCart(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceCartGetRes{Cart: toAPICommerceCart(cart)}, nil
}
