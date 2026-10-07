package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) ChannelSignedWebhookConfigurationUpdate(ctx context.Context, req *v1.ChannelSignedWebhookConfigurationUpdateReq) (res *v1.ChannelSignedWebhookConfigurationUpdateRes, err error) {
	secret := []byte(req.SigningSecret)
	req.SigningSecret = ""
	defer clear(secret)
	item, err := c.notifications.ConfigureChannel(ctx, notification.ChannelConfigurationInput{
		ChannelCode: "signed_webhook", ProviderCode: "signed_webhook_hmac_sha256_v1",
		NonSecret:       map[string]string{"endpoint_url": req.EndpointURL},
		Secrets:         map[string][]byte{"signing_secret": append([]byte(nil), secret...)},
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSignedWebhookConfigurationUpdateRes{Item: channelItem(*item)}, nil
}
