package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) RevokeToken(ctx context.Context, req *v1.RevokeTokenReq) (res *v1.RevokeTokenRes, err error) {
	return c.delegate.RevokeToken(ctx, req)
}
