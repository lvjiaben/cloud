package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelDisablementCreate(ctx context.Context, req *v1.ChannelDisablementCreateReq) (res *v1.ChannelDisablementCreateRes, err error) {
	item, err := c.notifications.DisableChannel(ctx, notification.ChannelLifecycleInput{ChannelCode: req.ChannelCode, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelDisablementCreateRes{Item: channelItem(*item)}, nil
}
