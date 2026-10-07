package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

func (c *ControllerV1) DeliveryGet(ctx context.Context, req *v1.DeliveryGetReq) (res *v1.DeliveryGetRes, err error) {
	item, err := c.notifications.GetDelivery(ctx, req.TaskNo)
	if err != nil {
		return nil, err
	}
	return &v1.DeliveryGetRes{Item: deliveryItem(*item)}, nil
}
