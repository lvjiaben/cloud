package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) CreateToken(ctx context.Context, req *v1.CreateTokenReq) (res *v1.CreateTokenRes, err error) {
	return c.delegate.CreateToken(ctx, req)
}
