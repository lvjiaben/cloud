package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) Lifecycle(ctx context.Context, req *v1.LifecycleReq) (res *v1.LifecycleRes, err error) {
	return c.delegate.Lifecycle(ctx, req)
}
