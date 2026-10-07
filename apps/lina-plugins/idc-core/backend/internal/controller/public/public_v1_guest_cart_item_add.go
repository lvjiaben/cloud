package public

import (
	"context"

	"lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/controller/cartcookie"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) GuestCartItemAdd(ctx context.Context, req *v1.GuestCartItemAddReq) (res *v1.GuestCartItemAddRes, err error) {
	token, err := cartcookie.Read(ctx)
	if err != nil {
		return nil, err
	}
	current, err := c.commerce.GetGuestCart(ctx, token)
	if err != nil {
		return nil, err
	}
	values, err := MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	cart, err := c.commerce.AddGuestItem(ctx, token, commerce.AddItemInput{
		ExpectedCartVersion: current.Version,
		Quantity:            req.Quantity,
		Purchase: catalog.PurchaseConfigurationInput{
			PublicSlug:            req.PublicSlug,
			ConfigurationRevision: req.ConfigurationRevision,
			BillingTermToken:      req.BillingTermToken,
			CurrencyCode:          req.CurrencyCode,
			PurchaseCycles:        req.PurchaseCycles,
			Values:                values,
		},
	})
	if err != nil {
		return nil, err
	}
	return &v1.GuestCartItemAddRes{Cart: guestCartReference(cart)}, nil
}
