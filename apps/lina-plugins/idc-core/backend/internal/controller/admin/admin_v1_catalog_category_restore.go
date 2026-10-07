// This file delegates category restoration to the transactional catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogCategoryRestore(ctx context.Context, req *v1.CatalogCategoryRestoreReq) (*v1.CatalogCategoryRestoreRes, error) {
	restored, err := c.catalogWriter.RestoreCategory(ctx, catalog.CategoryStateInput{CategoryNo: req.CategoryNo, Version: req.Version})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogCategoryRestoreRes{Category: mapCatalogCategory(catalog.CategoryManagementSummary{
		CategoryNo: restored.CategoryNo, ParentCategoryNo: restored.ParentCategoryNo,
		NameZhCN: restored.NameZhCN, NameEnUS: restored.NameEnUS,
		SortOrder: restored.SortOrder, Status: restored.Status, Version: restored.Version, Deleted: restored.Deleted,
	})}, nil
}
