package public

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/controller/cartcookie"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) GuestCartOpen(ctx context.Context, req *v1.GuestCartOpenReq) (res *v1.GuestCartOpenRes, err error) {
	token, err := cartcookie.Read(ctx)
	if err != nil {
		return nil, err
	}
	if token != "" {
		cart, cartErr := c.commerce.GetGuestCart(ctx, token)
		if cartErr == nil && cart != nil {
			if cart.CurrencyCode != req.CurrencyCode {
				cart, cartErr = c.commerce.SwitchGuestCurrency(ctx, token, req.CurrencyCode, cart.Version)
			}
			if cartErr == nil {
				return guestCartOpenResponse(cart), nil
			}
			return nil, cartErr
		}
		if !bizerr.Is(cartErr, commerce.CodeInvalid) && !bizerr.Is(cartErr, commerce.CodeNotAccessible) {
			return nil, cartErr
		}
		if err = cartcookie.Clear(ctx); err != nil {
			return nil, err
		}
	}
	created, err := c.commerce.CreateGuestCart(ctx, req.CurrencyCode)
	if err != nil {
		return nil, err
	}
	if err = cartcookie.Write(ctx, created.Token); err != nil {
		return nil, err
	}
	return guestCartOpenResponse(created.Cart), nil
}

func guestCartOpenResponse(cart *commerce.Cart) *v1.GuestCartOpenRes {
	return &v1.GuestCartOpenRes{Cart: guestCartReference(cart)}
}

func guestCartReference(cart *commerce.Cart) *v1.GuestCartReference {
	if cart == nil {
		return nil
	}
	return &v1.GuestCartReference{Reference: cart.CartNo}
}
