// This file delegates the plugin management read API to its startup-injected catalog.

package management

import (
	"context"

	"lina-plugin-plugin-manager/backend/api/management/v1"
)

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error) {
	return c.catalog.List(ctx, req)
}
