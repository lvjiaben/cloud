package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelConfigurationClearanceCreate(ctx context.Context, req *v1.ChannelConfigurationClearanceCreateReq) (res *v1.ChannelConfigurationClearanceCreateRes, err error) {
	item, err := c.notifications.ClearChannel(ctx, notification.ChannelLifecycleInput{ChannelCode: req.ChannelCode, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelConfigurationClearanceCreateRes{Item: channelItem(*item)}, nil
}
