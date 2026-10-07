// This file handles bounded automation-provider discovery and returns only safe selection projections.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationProviderList(ctx context.Context, req *v1.CatalogAutomationProviderListReq) (res *v1.CatalogAutomationProviderListRes, err error) {
	page, err := c.productAutomation.ListProviderCandidates(ctx, productautomation.PageInput{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CatalogAutomationProviderItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, &v1.CatalogAutomationProviderItem{
			SelectionToken: item.SelectionToken, DisplayName: item.DisplayName,
			InterfaceConfiguration: item.InterfaceConfiguration,
		})
	}
	return &v1.CatalogAutomationProviderListRes{Items: items, Total: page.Total}, nil
}
