package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

func (c *ControllerV1) ChannelList(ctx context.Context, req *v1.ChannelListReq) (res *v1.ChannelListRes, err error) {
	items, err := c.notifications.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.ChannelItem, len(items))
	for index := range items {
		result[index] = channelItem(items[index])
	}
	return &v1.ChannelListRes{Items: result}, nil
}
