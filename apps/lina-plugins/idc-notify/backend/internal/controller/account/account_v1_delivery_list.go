package account

import (
	"context"

	"lina-plugin-idc-notify/backend/api/account/v1"
)

func (c *ControllerV1) DeliveryList(ctx context.Context, req *v1.DeliveryListReq) (res *v1.DeliveryListRes, err error) {
	page, err := c.notifications.ListOwnDeliveries(ctx, req.Cursor, req.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.DeliveryItem, len(page.Items))
	for index := range page.Items {
		item := page.Items[index]
		items[index] = &v1.DeliveryItem{TaskNo: item.TaskNo, EventCode: item.EventCode,
			Topic: item.Topic, ChannelCode: item.ChannelCode, Status: string(item.Status),
			ResultClass: item.ResultClass, QueuedAt: item.QueuedAt.UnixMilli()}
		if item.CompletedAt != nil {
			value := item.CompletedAt.UnixMilli()
			items[index].CompletedAt = &value
		}
	}
	return &v1.DeliveryListRes{Items: items, NextCursor: page.NextCursor}, nil
}
