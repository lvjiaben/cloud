package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) Catalog(ctx context.Context, req *v1.CatalogReq) (res *v1.CatalogRes, err error) {
	return c.delegate.Catalog(ctx, req)
}
