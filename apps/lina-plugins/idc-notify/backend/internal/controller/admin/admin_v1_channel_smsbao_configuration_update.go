package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelSMSBaoConfigurationUpdate(ctx context.Context, req *v1.ChannelSMSBaoConfigurationUpdateReq) (res *v1.ChannelSMSBaoConfigurationUpdateRes, err error) {
	username, apiKey := []byte(req.Username), []byte(req.APIKey)
	req.Username, req.APIKey = "", ""
	item, err := c.notifications.ConfigureSMSBao(ctx, notification.SMSBaoConfigurationInput{
		SignName: req.SignName, ProductID: req.ProductID, Username: username, APIKey: apiKey,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSMSBaoConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
