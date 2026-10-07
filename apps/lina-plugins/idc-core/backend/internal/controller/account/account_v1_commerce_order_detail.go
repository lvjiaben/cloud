package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) CommerceOrderDetail(ctx context.Context, req *v1.CommerceOrderDetailReq) (res *v1.CommerceOrderDetailRes, err error) {
	order, err := c.orders.GetAccountOrder(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderDetailRes{Order: ToAPICommerceOrderDetail(order)}, nil
}
