package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) GetWebhook(ctx context.Context, req *v1.GetWebhookReq) (res *v1.GetWebhookRes, err error) {
	return c.delegate.GetWebhook(ctx, req)
}
