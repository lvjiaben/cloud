package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelWeChatOfficialConfigurationUpdate(ctx context.Context, req *v1.ChannelWeChatOfficialConfigurationUpdateReq) (res *v1.ChannelWeChatOfficialConfigurationUpdateRes, err error) {
	secret, callbackToken, encodingAESKey := []byte(req.AppSecret), []byte(req.CallbackToken), []byte(req.EncodingAESKey)
	req.AppSecret, req.CallbackToken, req.EncodingAESKey = "", "", ""
	item, err := c.notifications.ConfigureWeChatOfficial(ctx, notification.WeChatOfficialConfigurationInput{
		AppID: req.AppID, AppSecret: secret, CallbackToken: callbackToken, EncodingAESKey: encodingAESKey,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelWeChatOfficialConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
