package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) LifecycleOutput(ctx context.Context, req *v1.LifecycleOutputReq) (res *v1.LifecycleOutputRes, err error) {
	return c.delegate.LifecycleOutput(ctx, req)
}
