package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	accountcontroller "lina-plugin-idc-core/backend/internal/controller/account"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderDetail(ctx context.Context, req *v1.CommerceOrderDetailReq) (res *v1.CommerceOrderDetailRes, err error) {
	if c.commerceOrders == nil {
		return nil, bizerr.NewCode(commerce.CodeOrderDirectoryUnavailable)
	}
	order, err := c.commerceOrders.GetManagementOrder(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderDetailRes{
		Order:        accountcontroller.ToAPICommerceOrderDetail(order),
		ContractRule: toAPIOrderContractRule(order.Summary),
	}, nil
}
