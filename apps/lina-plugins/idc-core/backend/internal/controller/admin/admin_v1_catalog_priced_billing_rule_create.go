// This file maps exact operator-entered amounts into one atomic priced-period edit.

package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogPricedBillingRuleCreate(ctx context.Context, req *v1.CatalogPricedBillingRuleCreateReq) (res *v1.CatalogPricedBillingRuleCreateRes, err error) {
	tiers, err := catalogPriceScheduleTiers([]v1.CatalogPriceTierInput{{MinimumQuantity: 0, AmountMinor: req.AmountMinor, SetupFeeMinor: req.SetupFeeMinor, RenewalAmountMinor: req.RenewalAmountMinor}})
	if err != nil {
		return nil, err
	}
	rule, err := catalog.CreatePricedBillingRule(ctx, c.catalogWriter, c.catalogPublication, catalog.PricedBillingRuleCreateInput{VersionNo: req.VersionNo, RuleCode: req.RuleCode, PeriodUnit: catalog.PeriodUnit(req.PeriodUnit), PeriodCount: req.PeriodCount, MinimumPurchaseCycles: req.MinimumPurchaseCycles, MaximumRenewalCycles: req.MaximumRenewalCycles, CurrencyCode: req.CurrencyCode, Price: tiers[0]})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogPricedBillingRuleCreateRes{BillingRuleNo: rule.BillingRuleNo}, nil
}
