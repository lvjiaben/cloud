// This file delegates fulfillment-policy writes after exact plugin admission.

package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogFulfillmentPolicyUpdate(
	ctx context.Context,
	req *v1.CatalogFulfillmentPolicyUpdateReq,
) (*v1.CatalogFulfillmentPolicyUpdateRes, error) {
	if selected := req.UpstreamProduct; selected != nil {
		if c.catalogImport == nil || req.InterfaceInstanceNo == nil || *req.InterfaceInstanceNo == "" || len(selected.Prices) > 120 {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		prices := make([]resourceinterfaceprojectioncap.CatalogPrice, 0, len(selected.Prices))
		for _, price := range selected.Prices {
			prices = append(prices, resourceinterfaceprojectioncap.CatalogPrice{CurrencyCode: price.CurrencyCode, Period: resourceinterfaceprojectioncap.CatalogPeriod(price.Period), PeriodCount: price.PeriodCount, Amount: price.Amount, SetupFee: price.SetupFee})
		}
		_, err := c.catalogImport.ImportExisting(ctx, catalog.ImportInput{TargetVersionNo: req.VersionNo, RequestKey: selected.RequestKey, InterfaceInstanceNo: *req.InterfaceInstanceNo, Reference: selected.Reference, CategoryNo: selected.CategoryNo, PricingMode: catalog.ImportPricingMode(selected.PricingMode), MarkupPercent: selected.MarkupPercent, Prices: prices}, c.catalogPublication, c.productAutomation, req.StrategyCode)
		if err != nil {
			return nil, err
		}
		return &v1.CatalogFulfillmentPolicyUpdateRes{}, nil
	}
	result, err := productautomation.SaveFulfillmentSettings(ctx, c.productAutomation, c.catalogPublication, productautomation.FulfillmentSettingsInput{VersionNo: req.VersionNo, StrategyCode: req.StrategyCode, InterfaceInstanceNo: req.InterfaceInstanceNo})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogFulfillmentPolicyUpdateRes{Policy: mapFulfillmentPolicySummary(result.Policy, result.Contract)}, nil
}
