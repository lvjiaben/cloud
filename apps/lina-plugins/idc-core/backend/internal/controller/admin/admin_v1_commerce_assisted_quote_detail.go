package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CommerceAssistedQuoteDetail(ctx context.Context, req *v1.CommerceAssistedQuoteDetailReq) (res *v1.CommerceAssistedQuoteDetailRes, err error) {
	request, err := c.commerceAssistedQuotes.GetAssistedQuote(ctx, req.RequestNo)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceAssistedQuoteDetailRes{Request: toAPICommerceAssistedQuote(request)}, nil
}
