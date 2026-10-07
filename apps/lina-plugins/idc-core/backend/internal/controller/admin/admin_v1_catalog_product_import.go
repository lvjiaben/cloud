// This file delegates explicit upstream draft import to the owner orchestrator.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductImport(ctx context.Context, req *v1.CatalogProductImportReq) (res *v1.CatalogProductImportRes, err error) {
	if c.catalogImport == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	if len(req.Prices) > 120 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	prices := make([]resourceinterfaceprojectioncap.CatalogPrice, 0, len(req.Prices))
	for _, price := range req.Prices {
		prices = append(prices, resourceinterfaceprojectioncap.CatalogPrice{CurrencyCode: price.CurrencyCode, Period: resourceinterfaceprojectioncap.CatalogPeriod(price.Period), PeriodCount: price.PeriodCount, Amount: price.Amount, SetupFee: price.SetupFee})
	}
	result, err := c.catalogImport.Import(ctx, catalog.ImportInput{RequestKey: req.RequestKey, InterfaceInstanceNo: req.InterfaceInstanceNo, Reference: req.Reference, CategoryNo: req.CategoryNo,
		NameZhCN: req.NameZhCn, NameEnUS: req.NameEnUs, PricingMode: catalog.ImportPricingMode(req.PricingMode), MarkupPercent: req.MarkupPercent, Prices: prices})
	if err != nil {
		return nil, err
	}
	missing := make([]v1.CatalogImportMissingConfiguration, 0, len(result.MissingConfiguration))
	for _, issue := range result.MissingConfiguration {
		missing = append(missing, v1.CatalogImportMissingConfiguration{FieldKey: issue.FieldKey, NameZhCn: issue.NameZhCN, NameEnUs: issue.NameEnUS})
	}
	item := result.Item
	return &v1.CatalogProductImportRes{Item: &v1.CatalogProductCreateRes{ProductNo: item.ProductNo, VersionNo: item.VersionNo, VersionNumber: item.VersionNumber, CategoryNo: item.CategoryNo,
		Status: item.Status, VersionStatus: item.VersionStatus, Source: item.Source}, MissingConfiguration: missing, PricingIncomplete: result.PricingIncomplete}, nil
}
