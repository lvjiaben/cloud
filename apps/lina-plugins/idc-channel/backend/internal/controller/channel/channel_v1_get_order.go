package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) GetOrder(ctx context.Context, req *v1.GetOrderReq) (res *v1.GetOrderRes, err error) {
	return c.delegate.GetOrder(ctx, req)
}
