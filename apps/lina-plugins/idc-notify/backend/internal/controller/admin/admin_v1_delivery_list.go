package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) DeliveryList(ctx context.Context, req *v1.DeliveryListReq) (res *v1.DeliveryListRes, err error) {
	page, err := c.notifications.ListDeliveries(ctx, notification.DeliveryListInput{ChannelCode: req.ChannelCode, Status: req.Status, AfterTaskNo: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.DeliveryItem, len(page.Items))
	for index := range page.Items {
		items[index] = deliveryItem(page.Items[index])
	}
	return &v1.DeliveryListRes{Items: items, NextCursor: page.NextCursor}, nil
}
