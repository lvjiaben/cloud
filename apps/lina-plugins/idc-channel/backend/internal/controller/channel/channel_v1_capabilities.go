package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) Capabilities(ctx context.Context, req *v1.CapabilitiesReq) (res *v1.CapabilitiesRes, err error) {
	return c.delegate.Capabilities(ctx, req)
}
