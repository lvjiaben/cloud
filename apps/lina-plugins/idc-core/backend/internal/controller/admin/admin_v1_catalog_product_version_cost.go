// This file maps product-version cost management requests to the catalog
// service and keeps time and amount conversion at the HTTP boundary.
package admin

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// CatalogProductVersionCostList returns bounded cost settings for one version.
func (c *ControllerV1) CatalogProductVersionCostList(ctx context.Context, req *v1.CatalogProductVersionCostListReq) (*v1.CatalogProductVersionCostListRes, error) {
	rows, err := c.catalogWriter.ListProductVersionCosts(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	result := &v1.CatalogProductVersionCostListRes{Costs: make([]*v1.CatalogProductVersionCostItem, len(rows))}
	for i := range rows {
		result.Costs[i] = mapCatalogProductVersionCost(rows[i])
	}
	return result, nil
}

// CatalogProductVersionCostCreate creates one authorized cost setting.
func (c *ControllerV1) CatalogProductVersionCostCreate(ctx context.Context, req *v1.CatalogProductVersionCostCreateReq) (*v1.CatalogProductVersionCostCreateRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogProductVersionCostCreateRes, error) {
		amount, start, end, err := parseCatalogProductVersionCost(req.UnitCostMinor, req.EffectiveFrom, req.EffectiveUntil)
		if err != nil {
			return nil, err
		}
		row, err := c.catalogWriter.CreateProductVersionCost(ctx, catalog.ProductVersionCostCreateInput{VersionNo: req.VersionNo, CurrencyCode: req.CurrencyCode, PeriodUnit: catalog.PeriodUnit(req.PeriodUnit), PeriodCount: req.PeriodCount, UnitCostMinor: amount, EffectiveFrom: start, EffectiveUntil: end})
		if err != nil {
			return nil, err
		}
		return &v1.CatalogProductVersionCostCreateRes{Cost: mapCatalogProductVersionCost(*row)}, nil
	})
}

// CatalogProductVersionCostUpdate updates one setting at an expected version.
func (c *ControllerV1) CatalogProductVersionCostUpdate(ctx context.Context, req *v1.CatalogProductVersionCostUpdateReq) (*v1.CatalogProductVersionCostUpdateRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogProductVersionCostUpdateRes, error) {
		amount, start, end, err := parseCatalogProductVersionCost(req.UnitCostMinor, req.EffectiveFrom, req.EffectiveUntil)
		if err != nil {
			return nil, err
		}
		row, err := c.catalogWriter.UpdateProductVersionCost(ctx, catalog.ProductVersionCostUpdateInput{ProductVersionCostCreateInput: catalog.ProductVersionCostCreateInput{VersionNo: req.VersionNo, CurrencyCode: req.CurrencyCode, PeriodUnit: catalog.PeriodUnit(req.PeriodUnit), PeriodCount: req.PeriodCount, UnitCostMinor: amount, EffectiveFrom: start, EffectiveUntil: end}, CostNo: req.CostNo, ExpectedVersion: req.ExpectedVersion})
		if err != nil {
			return nil, err
		}
		return &v1.CatalogProductVersionCostUpdateRes{Cost: mapCatalogProductVersionCost(*row)}, nil
	})
}

func parseCatalogProductVersionCost(amountText string, startMillis, endMillis *int64) (int64, *time.Time, *time.Time, error) {
	amount, err := parseCatalogAmount(amountText)
	if err != nil || amount < 0 {
		return 0, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	var start, end *time.Time
	if startMillis != nil {
		value := time.UnixMilli(*startMillis).UTC()
		start = &value
	}
	if endMillis != nil {
		value := time.UnixMilli(*endMillis).UTC()
		end = &value
	}
	return amount, start, end, nil
}

func mapCatalogProductVersionCost(row catalog.ProductVersionCost) *v1.CatalogProductVersionCostItem {
	item := &v1.CatalogProductVersionCostItem{CostNo: row.CostNo, CurrencyCode: row.CurrencyCode, PeriodUnit: string(row.PeriodUnit), PeriodCount: row.PeriodCount, UnitCostMinor: formatCatalogAmount(row.UnitCostMinor), EffectiveFrom: row.EffectiveFrom.UnixMilli(), Version: row.Version}
	if row.EffectiveUntil != nil {
		value := row.EffectiveUntil.UnixMilli()
		item.EffectiveUntil = &value
	}
	return item
}
