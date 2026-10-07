package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelWeComConfigurationUpdate(ctx context.Context, req *v1.ChannelWeComConfigurationUpdateReq) (res *v1.ChannelWeComConfigurationUpdateRes, err error) {
	secret := []byte(req.AppSecret)
	req.AppSecret = ""
	item, err := c.notifications.ConfigureWeCom(ctx, notification.WeComConfigurationInput{
		CorpID: req.CorpID, AgentID: req.AgentID, AppSecret: secret,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelWeComConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
