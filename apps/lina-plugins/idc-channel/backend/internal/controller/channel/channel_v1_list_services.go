package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) ListServices(ctx context.Context, req *v1.ListServicesReq) (*v1.ListServicesRes, error) {
	return c.delegate.ListServices(ctx, req)
}
