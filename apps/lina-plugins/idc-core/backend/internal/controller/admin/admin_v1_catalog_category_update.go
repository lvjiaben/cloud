// This file delegates category updates to the transactional catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogCategoryUpdate(ctx context.Context, req *v1.CatalogCategoryUpdateReq) (*v1.CatalogCategoryUpdateRes, error) {
	updated, err := c.catalogWriter.UpdateCategory(ctx, catalog.CategoryUpdateInput{
		CategoryNo: req.CategoryNo, ParentCategoryNo: req.ParentCategoryNo,
		NameZhCN: req.NameZhCn, NameEnUS: req.NameEnUs, SortOrder: req.SortOrder, Version: req.Version,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogCategoryUpdateRes{Category: mapCatalogCategory(catalog.CategoryManagementSummary{
		CategoryNo: updated.CategoryNo, ParentCategoryNo: updated.ParentCategoryNo,
		NameZhCN: updated.NameZhCN, NameEnUS: updated.NameEnUS,
		SortOrder: updated.SortOrder, Status: updated.Status, Version: updated.Version, Deleted: updated.Deleted,
	})}, nil
}
