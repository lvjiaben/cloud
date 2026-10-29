package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	publiccontroller "lina-plugin-idc-core/backend/internal/controller/public"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceCartItemAdd(ctx context.Context, req *v1.CommerceCartItemAddReq) (res *v1.CommerceCartItemAddRes, err error) {
	values, err := publiccontroller.MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	cart, err := c.commerce.AddAccountItem(ctx, commerce.AddItemInput{
		ExpectedCartVersion: req.ExpectedCartVersion, Quantity: req.Quantity,
		Purchase: catalog.PurchaseConfigurationInput{ServerLoginToken: req.ServerLoginToken, PublicSlug: req.PublicSlug,
			ConfigurationRevision: req.ConfigurationRevision, BillingTermToken: req.BillingTermToken,
			CurrencyCode: req.CurrencyCode, PurchaseCycles: req.PurchaseCycles, Values: values},
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceCartItemAddRes{Cart: toAPICommerceCart(cart)}, nil
}
