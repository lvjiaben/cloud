package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelAliyunSMSConfigurationUpdate(ctx context.Context, req *v1.ChannelAliyunSMSConfigurationUpdateReq) (res *v1.ChannelAliyunSMSConfigurationUpdateRes, err error) {
	accessKeyID, accessKeySecret := []byte(req.AccessKeyID), []byte(req.AccessKeySecret)
	req.AccessKeyID, req.AccessKeySecret = "", ""
	item, err := c.notifications.ConfigureAliyunSMS(ctx, notification.AliyunSMSConfigurationInput{
		SignName: req.SignName, AccessKeyID: accessKeyID, AccessKeySecret: accessKeySecret,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelAliyunSMSConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
