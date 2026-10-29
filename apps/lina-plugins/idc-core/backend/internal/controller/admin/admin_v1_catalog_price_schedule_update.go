// This file delegates complete draft price replacements to the catalog owner.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CatalogPriceScheduleUpdate(ctx context.Context, req *v1.CatalogPriceScheduleUpdateReq) (*v1.CatalogPriceScheduleUpdateRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogPriceScheduleUpdateRes, error) {
		return c.savePriceSchedule(ctx, &v1.CatalogPriceScheduleCreateReq{VersionNo: req.VersionNo, BillingRuleNo: req.BillingRuleNo, ConfigurationPriceDimensionNo: req.ConfigurationPriceDimensionNo, CurrencyCode: req.CurrencyCode, Tiers: req.Tiers, Status: req.Status}, req.PriceNo)
	})
}
