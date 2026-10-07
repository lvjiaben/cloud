package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/controller/cartcookie"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceGuestCartMerge(ctx context.Context, _ *v1.CommerceGuestCartMergeReq) (res *v1.CommerceGuestCartMergeRes, err error) {
	token, err := cartcookie.Read(ctx)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return &v1.CommerceGuestCartMergeRes{Merged: false}, nil
	}
	cart, err := c.commerce.MergeGuestCart(ctx, token)
	if err != nil {
		if !bizerr.Is(err, commerce.CodeInvalid) && !bizerr.Is(err, commerce.CodeNotAccessible) {
			return nil, err
		}
		if clearErr := cartcookie.Clear(ctx); clearErr != nil {
			return nil, clearErr
		}
		return &v1.CommerceGuestCartMergeRes{Merged: false}, nil
	}
	if err = cartcookie.Clear(ctx); err != nil {
		return nil, err
	}
	return &v1.CommerceGuestCartMergeRes{Merged: true, Cart: toAPICommerceCart(cart)}, nil
}
