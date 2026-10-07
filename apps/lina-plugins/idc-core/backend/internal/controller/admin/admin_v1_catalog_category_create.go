// This file delegates category creation to the transactional catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogCategoryCreate(
	ctx context.Context,
	req *v1.CatalogCategoryCreateReq,
) (*v1.CatalogCategoryCreateRes, error) {
	created, err := c.catalogWriter.CreateCategory(ctx, catalog.CategoryCreateInput{
		ParentCategoryNo: req.ParentCategoryNo,
		NameZhCN:         req.NameZhCn, NameEnUS: req.NameEnUs, SortOrder: req.SortOrder,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogCategoryCreateRes{Category: mapCatalogCategory(catalog.CategoryManagementSummary{
		CategoryNo: created.CategoryNo, ParentCategoryNo: created.ParentCategoryNo,
		NameZhCN: created.NameZhCN, NameEnUS: created.NameEnUS,
		SortOrder: created.SortOrder, Status: created.Status, Version: created.Version, Deleted: created.Deleted,
	})}, nil
}
