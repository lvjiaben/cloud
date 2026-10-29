// This file projects bounded guest cart facts after cookie ownership validation.
package public

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/controller/cartcookie"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

// GuestCartGet never accepts a cart number or raw ownership token from a query.
func (c *ControllerV1) GuestCartGet(ctx context.Context, _ *v1.GuestCartGetReq) (*v1.GuestCartGetRes, error) {
	token, err := cartcookie.Read(ctx)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return &v1.GuestCartGetRes{}, nil
	}
	cart, err := c.commerce.GetGuestCart(ctx, token)
	if bizerr.Is(err, commerce.CodeInvalid) || bizerr.Is(err, commerce.CodeNotAccessible) {
		if err = cartcookie.Clear(ctx); err != nil {
			return nil, err
		}
		return &v1.GuestCartGetRes{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &v1.GuestCartGetRes{Cart: guestCartView(cart)}, nil
}

// guestCartView intentionally excludes configuration contents, files and identities.
func guestCartView(cart *commerce.Cart) *v1.GuestCartView {
	if cart == nil {
		return nil
	}
	view := &v1.GuestCartView{Reference: cart.CartNo, CurrencyCode: cart.CurrencyCode, Version: cart.Version, Items: make([]*v1.GuestCartViewItem, 0, len(cart.Items))}
	for _, item := range cart.Items {
		view.Items = append(view.Items, &v1.GuestCartViewItem{CartItemNo: item.CartItemNo, ConfigurationReference: item.ConfigurationReference, ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS, PublicSlug: item.PublicSlug, Quantity: item.Quantity, PurchaseCycles: item.PurchaseCycles, Version: item.Version})
	}
	return view
}
