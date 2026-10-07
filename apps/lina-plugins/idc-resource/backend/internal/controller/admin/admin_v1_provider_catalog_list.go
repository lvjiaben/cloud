// This file adapts the bounded staff-only supplier list without importing data.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/api/admin/v1"
)

func (c *ControllerV1) ProviderCatalogList(ctx context.Context, req *v1.ProviderCatalogListReq) (res *v1.ProviderCatalogListRes, err error) {
	page, err := c.catalog.ListCatalog(ctx, req.InterfaceNo, resourceinterfaceprojectioncap.CatalogQuery{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderCatalogListRes{Items: page.Items, Total: page.Total}, nil
}
