// This file delegates the plugin management read API to its startup-injected catalog.

package management

import (
	"context"

	"lina-plugin-plugin-manager/backend/api/management/v1"
)

func (c *ControllerV1) Categories(ctx context.Context, req *v1.CategoriesReq) (res *v1.CategoriesRes, err error) {
	return &v1.CategoriesRes{List: c.catalog.Categories(ctx)}, nil
}
