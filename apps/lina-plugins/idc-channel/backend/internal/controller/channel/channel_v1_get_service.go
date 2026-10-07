package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) GetService(ctx context.Context, req *v1.GetServiceReq) (*v1.GetServiceRes, error) {
	return c.delegate.GetService(ctx, req)
}
