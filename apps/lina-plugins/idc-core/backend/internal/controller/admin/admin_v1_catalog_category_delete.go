// This file delegates recoverable category deletion to the transactional service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogCategoryDelete(ctx context.Context, req *v1.CatalogCategoryDeleteReq) (*v1.CatalogCategoryDeleteRes, error) {
	if err := c.catalogWriter.DeleteCategory(ctx, catalog.CategoryStateInput{CategoryNo: req.CategoryNo, Version: req.Version}); err != nil {
		return nil, err
	}
	return &v1.CatalogCategoryDeleteRes{}, nil
}
