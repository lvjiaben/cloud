package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) ListTokens(ctx context.Context, req *v1.ListTokensReq) (res *v1.ListTokensRes, err error) {
	return c.delegate.ListTokens(ctx, req)
}
