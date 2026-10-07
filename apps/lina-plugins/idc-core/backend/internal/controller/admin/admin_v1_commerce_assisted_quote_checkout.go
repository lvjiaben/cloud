package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceAssistedQuoteCheckout(ctx context.Context, req *v1.CommerceAssistedQuoteCheckoutReq) (res *v1.CommerceAssistedQuoteCheckoutRes, err error) {
	request, err := c.commerceAssistedQuotes.CheckoutAssistedQuote(ctx, commerce.AssistedQuoteCheckoutInput{
		RequestNo: req.RequestNo, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceAssistedQuoteCheckoutRes{Request: toAPICommerceAssistedQuote(request)}, nil
}
