package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceAssistedQuoteList(ctx context.Context, req *v1.CommerceAssistedQuoteListReq) (res *v1.CommerceAssistedQuoteListRes, err error) {
	page, err := c.commerceAssistedQuotes.ListAssistedQuotes(ctx, commerce.AssistedQuoteFilter{
		CustomerNo: req.CustomerNo, StatusCode: commerce.AssistedQuoteStatus(req.StatusCode),
		Cursor: req.Cursor, Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	requests := make([]*v1.CommerceAssistedQuote, 0, len(page.Requests))
	for index := range page.Requests {
		requests = append(requests, toAPICommerceAssistedQuote(&page.Requests[index]))
	}
	return &v1.CommerceAssistedQuoteListRes{Requests: requests, NextCursor: page.NextCursor, Total: page.Total}, nil
}
