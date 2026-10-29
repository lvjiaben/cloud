// This file handles bounded, server-filtered automation-interface listing.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceList(ctx context.Context, req *v1.CatalogAutomationInterfaceListReq) (res *v1.CatalogAutomationInterfaceListRes, err error) {
	page, err := c.productAutomation.ListInterfaces(ctx, productautomation.PageInput{
		Page: req.Page, PageSize: req.PageSize, NamePrefix: req.NamePrefix, ForProductBinding: req.ForProductBinding,
		NewBusinessState: req.NewBusinessState, ProviderSelectionToken: req.ProviderSelectionToken,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CatalogAutomationInterfaceItem, 0, len(page.Items))
	for _, item := range page.Items {
		value := item
		items = append(items, mapAutomationInterface(&value))
	}
	return &v1.CatalogAutomationInterfaceListRes{Items: items, Total: page.Total}, nil
}
