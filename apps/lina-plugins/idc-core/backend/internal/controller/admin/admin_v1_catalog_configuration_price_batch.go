package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogConfigurationPriceBatch(ctx context.Context, req *v1.CatalogConfigurationPriceBatchReq) (*v1.CatalogConfigurationPriceBatchRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogConfigurationPriceBatchRes, error) {
		rows := make([]catalog.ConfigurationPriceBatchRow, 0, len(req.Rows))
		for _, row := range req.Rows {
			tiers, err := catalogPriceScheduleTiers(row.Tiers)
			if err != nil {
				return nil, err
			}
			rows = append(rows, catalog.ConfigurationPriceBatchRow{TargetKind: catalog.PriceBatchTargetKind(row.TargetKind), CandidateToken: row.CandidateToken, PreviousPriceNo: row.PreviousPriceNo, Schedule: catalog.PriceScheduleCreateInput{VersionNo: req.VersionNo, BillingRuleNo: row.BillingRuleNo, CurrencyCode: row.CurrencyCode, Tiers: tiers, Active: row.Active == nil || *row.Active}})
		}
		if err := c.catalogWriter.SaveConfigurationPriceBatch(ctx, req.VersionNo, rows, c.productAutomation); err != nil {
			return nil, err
		}
		return &v1.CatalogConfigurationPriceBatchRes{Saved: len(rows)}, nil
	})
}
