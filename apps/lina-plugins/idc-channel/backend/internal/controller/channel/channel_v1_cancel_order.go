package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) CancelOrder(ctx context.Context, req *v1.CancelOrderReq) (res *v1.CancelOrderRes, err error) {
	return c.delegate.CancelOrder(ctx, req)
}
