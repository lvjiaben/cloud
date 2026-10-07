// This file maps authorized bounded order pages to the staff API contract.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	accountcontroller "lina-plugin-idc-core/backend/internal/controller/account"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderList(ctx context.Context, req *v1.CommerceOrderListReq) (res *v1.CommerceOrderListRes, err error) {
	if c.commerceOrders == nil {
		return nil, bizerr.NewCode(commerce.CodeOrderDirectoryUnavailable)
	}
	page, err := c.commerceOrders.ListManagementOrders(ctx, commerce.OrderDirectoryFilter{
		CustomerNo: req.CustomerNo, StatusCode: req.StatusCode,
		RiskDecisionCode: req.RiskDecisionCode, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderListRes{Orders: accountcontroller.ToAPICommerceOrderSummaries(page.Orders), NextCursor: page.NextCursor, Total: page.Total}, nil
}
