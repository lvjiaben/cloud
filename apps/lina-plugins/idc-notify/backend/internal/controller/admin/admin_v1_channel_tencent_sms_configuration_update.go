package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelTencentSMSConfigurationUpdate(ctx context.Context, req *v1.ChannelTencentSMSConfigurationUpdateReq) (res *v1.ChannelTencentSMSConfigurationUpdateRes, err error) {
	secretID, secretKey := []byte(req.SecretID), []byte(req.SecretKey)
	req.SecretID, req.SecretKey = "", ""
	item, err := c.notifications.ConfigureTencentSMS(ctx, notification.TencentSMSConfigurationInput{
		Region: req.Region, SmsSdkAppID: req.SmsSdkAppID, SignName: req.SignName,
		SecretID: secretID, SecretKey: secretKey, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelTencentSMSConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
