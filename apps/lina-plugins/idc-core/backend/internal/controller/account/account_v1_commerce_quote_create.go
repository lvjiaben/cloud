package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceQuoteCreate(ctx context.Context, req *v1.CommerceQuoteCreateReq) (res *v1.CommerceQuoteCreateRes, err error) {
	quote, err := c.commerce.QuoteAccountCart(ctx, commerce.QuoteRequest{TargetEntitlementNo: req.TargetEntitlementNo, ExpectedCartVersion: req.ExpectedCartVersion, AssetNumbers: req.AssetNumbers})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceQuoteCreateRes{Quote: toAPICommerceQuote(quote)}, nil
}
