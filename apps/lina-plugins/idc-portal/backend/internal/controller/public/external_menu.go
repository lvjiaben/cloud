package public

import (
	"context"

	"lina-plugin-idc-portal/backend/api/public/v1"
)

func (c *ControllerV1) ExternalMenuList(ctx context.Context, _ *v1.ExternalMenuListReq) (*v1.ExternalMenuListRes, error) {
	items, err := c.externalMenus.List(ctx, true)
	return &v1.ExternalMenuListRes{Items: items}, err
}
