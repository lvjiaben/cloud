package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommerceCartItemRemove(ctx context.Context, req *v1.CommerceCartItemRemoveReq) (res *v1.CommerceCartItemRemoveRes, err error) {
	cart, err := c.commerce.RemoveAccountItem(ctx, req.CartItemNo, req.ExpectedCartVersion, req.ExpectedItemVersion)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceCartItemRemoveRes{Cart: toAPICommerceCart(cart)}, nil
}
