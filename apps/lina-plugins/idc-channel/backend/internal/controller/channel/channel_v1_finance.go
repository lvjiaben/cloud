package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) Finance(ctx context.Context, req *v1.FinanceReq) (*v1.FinanceRes, error) {
	return c.delegate.Finance(ctx, req)
}
