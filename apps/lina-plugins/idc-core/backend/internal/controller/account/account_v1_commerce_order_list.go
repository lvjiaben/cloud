package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderList(ctx context.Context, req *v1.CommerceOrderListReq) (res *v1.CommerceOrderListRes, err error) {
	page, err := c.orders.ListAccountOrders(ctx, commerce.OrderDirectoryFilter{StatusCode: req.StatusCode, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderListRes{Orders: toAPICommerceOrderSummaries(page.Orders), NextCursor: page.NextCursor}, nil
}
