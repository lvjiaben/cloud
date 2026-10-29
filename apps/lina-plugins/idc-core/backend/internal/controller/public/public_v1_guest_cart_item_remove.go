// This file removes only the optimistic, cookie-owned anonymous purchase intent.
package public

import (
	"context"
	"lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/controller/cartcookie"
)

// GuestCartItemRemove delegates ownership and optimistic version checks to commerce.
func (c *ControllerV1) GuestCartItemRemove(ctx context.Context, req *v1.GuestCartItemRemoveReq) (*v1.GuestCartItemRemoveRes, error) {
	token, err := cartcookie.Read(ctx)
	if err != nil {
		return nil, err
	}
	cart, err := c.commerce.RemoveGuestItem(ctx, token, req.CartItemNo, req.ExpectedCartVersion, req.ExpectedItemVersion)
	if err != nil {
		return nil, err
	}
	return &v1.GuestCartItemRemoveRes{Cart: guestCartView(cart)}, nil
}
