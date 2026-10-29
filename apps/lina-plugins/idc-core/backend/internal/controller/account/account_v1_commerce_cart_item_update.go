// This file maps the governed owned-cart update contract to the commerce service.
package account

import (
	"context"
	"lina-plugin-idc-core/backend/api/account/v1"
	publiccontroller "lina-plugin-idc-core/backend/internal/controller/public"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

// CommerceCartItemUpdate updates quantity or complete configuration atomically.
func (c *ControllerV1) CommerceCartItemUpdate(ctx context.Context, req *v1.CommerceCartItemUpdateReq) (*v1.CommerceCartItemUpdateRes, error) {
	input := commerce.UpdateItemInput{CartItemNo: req.CartItemNo, ExpectedCartVersion: req.ExpectedCartVersion, ExpectedItemVersion: req.ExpectedItemVersion, Quantity: req.Quantity}
	if req.Purchase != nil {
		values, err := publiccontroller.MapPurchaseConfigurationInput(req.Purchase.Values)
		if err != nil {
			return nil, err
		}
		input.Purchase = &catalog.PurchaseConfigurationInput{ServerLoginToken: req.Purchase.ServerLoginToken, PublicSlug: req.Purchase.PublicSlug, ConfigurationRevision: req.Purchase.ConfigurationRevision, BillingTermToken: req.Purchase.BillingTermToken, CurrencyCode: req.Purchase.CurrencyCode, PurchaseCycles: req.Purchase.PurchaseCycles, Values: values}
	}
	cart, err := c.commerce.UpdateAccountItem(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceCartItemUpdateRes{Cart: toAPICommerceCart(cart)}, nil
}
