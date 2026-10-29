// This file returns bounded purchased services and customer-safe order links.

package directoryaccount

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryaccount/v1"
)

func (c *ControllerV1) Services(ctx context.Context, req *v1.ServicesReq) (*v1.ServicesRes, error) {
	page, err := c.service.ListProviderServices(ctx, req.DirectoryNo, req.Page, req.PageSize, req.Keyword, req.ProviderID)
	if err != nil {
		return nil, err
	}
	items := make([]v1.Entitlement, len(page.Items))
	for i, item := range page.Items {
		items[i] = v1.Entitlement{EntitlementOwnerID: item.EntitlementOwnerID, EntitlementNo: item.EntitlementNo, ProductNo: item.ProductNo, ProductNameZhCn: item.ProductNameZhCn, ProductNameEnUs: item.ProductNameEnUs, PluginId: item.PluginId, StatusCode: item.StatusCode, RequiresManualProcessing: item.RequiresManualProcessing, ManagementPreparation: item.ManagementPreparation, OrderNo: item.OrderNo, StartedAt: item.StartedAt, ExpiresAt: item.ExpiresAt}
		items[i].CustomerRemark = item.CustomerRemark
		items[i].CustomerRemarkRevision = item.CustomerRemarkRevision
		items[i].CurrentPriceAmountMinor = item.CurrentPriceAmountMinor
		items[i].CurrencyCode = item.CurrencyCode
		items[i].PeriodUnitCode = item.PeriodUnitCode
		items[i].PeriodCount = item.PeriodCount
		items[i].ChargeCycles = item.ChargeCycles
	}
	providers := make([]v1.ServiceProvider, len(page.Providers))
	for i, provider := range page.Providers {
		providers[i] = v1.ServiceProvider{PluginID: provider.PluginID, Total: provider.Total}
	}
	return &v1.ServicesRes{Providers: providers, Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}
