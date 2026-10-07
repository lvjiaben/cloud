// This file maps the bounded category selector to the admin API.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogCategoryList(
	ctx context.Context,
	req *v1.CatalogCategoryListReq,
) (*v1.CatalogCategoryListRes, error) {
	categories, err := c.catalogManagement.ListCategories(ctx, req.DeletionState)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CatalogCategoryItem, 0, len(categories))
	for _, category := range categories {
		items = append(items, mapCatalogCategory(category))
	}
	return &v1.CatalogCategoryListRes{Categories: items}, nil
}

func mapCatalogCategory(category catalog.CategoryManagementSummary) *v1.CatalogCategoryItem {
	return &v1.CatalogCategoryItem{
		CategoryNo: category.CategoryNo, ParentCategoryNo: category.ParentCategoryNo,
		NameZhCn: category.NameZhCN,
		NameEnUs: category.NameEnUS, SortOrder: category.SortOrder, Status: category.Status,
		Version: category.Version, Deleted: category.Deleted,
	}
}
