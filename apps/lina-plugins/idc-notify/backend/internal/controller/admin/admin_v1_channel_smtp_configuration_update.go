package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelSMTPConfigurationUpdate(ctx context.Context, req *v1.ChannelSMTPConfigurationUpdateReq) (res *v1.ChannelSMTPConfigurationUpdateRes, err error) {
	password := []byte(req.Password)
	req.Password = ""
	item, err := c.notifications.ConfigureSMTP(ctx, notification.SMTPConfigurationInput{
		Host: req.Host, Port: req.Port, Security: req.Security, Username: req.Username,
		Password: password, FromAddress: req.FromAddress, FromName: req.FromName,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSMTPConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
