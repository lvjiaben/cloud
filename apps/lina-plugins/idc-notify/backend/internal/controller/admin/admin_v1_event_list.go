package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

func (c *ControllerV1) EventList(ctx context.Context, req *v1.EventListReq) (res *v1.EventListRes, err error) {
	items, err := c.notifications.ListEvents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.EventItem, len(items))
	for index := range items {
		result[index] = eventItem(items[index])
	}
	return &v1.EventListRes{Items: result}, nil
}
