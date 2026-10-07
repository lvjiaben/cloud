package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) SettleOrder(ctx context.Context, req *v1.SettleOrderReq) (*v1.SettleOrderRes, error) {
	return c.delegate.SettleOrder(ctx, req)
}
