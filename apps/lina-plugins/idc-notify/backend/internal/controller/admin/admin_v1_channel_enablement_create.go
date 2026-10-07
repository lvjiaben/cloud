package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelEnablementCreate(ctx context.Context, req *v1.ChannelEnablementCreateReq) (res *v1.ChannelEnablementCreateRes, err error) {
	item, err := c.notifications.EnableChannel(ctx, notification.ChannelLifecycleInput{ChannelCode: req.ChannelCode, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelEnablementCreateRes{Item: channelItem(*item)}, nil
}
