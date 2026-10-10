// This file returns bounded purchased services and customer-safe order links.
package directoryaccount

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryaccount/v1"
)

func (c *ControllerV1) Services(ctx context.Context, req *v1.ServicesReq) (*v1.ServicesRes, error) {
	page, err := c.service.ListServices(ctx, req.DirectoryNo, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]v1.Entitlement, len(page.Items))
	for i, item := range page.Items {
		items[i] = v1.Entitlement{EntitlementNo: item.EntitlementNo, ProductNo: item.ProductNo, ProductNameZhCn: item.ProductNameZhCn, ProductNameEnUs: item.ProductNameEnUs, PluginId: item.PluginId, StatusCode: item.StatusCode, OrderNo: item.OrderNo, StartedAt: item.StartedAt, ExpiresAt: item.ExpiresAt}
	}
	return &v1.ServicesRes{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}
