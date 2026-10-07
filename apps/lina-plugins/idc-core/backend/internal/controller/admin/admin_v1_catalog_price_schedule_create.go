// This file delegates complete price schedules to the transactional catalog service.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogPriceScheduleCreate(
	ctx context.Context,
	req *v1.CatalogPriceScheduleCreateReq,
) (*v1.CatalogPriceScheduleCreateRes, error) {
	tiers := make([]catalog.PriceTierDefinition, len(req.Tiers))
	for index, tier := range req.Tiers {
		amountMinor, parseErr := parseCatalogAmount(tier.AmountMinor)
		if parseErr != nil {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		setupFeeMinor, parseErr := parseCatalogAmount(tier.SetupFeeMinor)
		if parseErr != nil {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		renewalAmountMinor, renewalErr := parseOptionalCatalogAmount(tier.RenewalAmountMinor)
		if renewalErr != nil {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		tiers[index] = catalog.PriceTierDefinition{
			MinimumQuantity: tier.MinimumQuantity, MaximumQuantity: tier.MaximumQuantity,
			AmountMinor: amountMinor, SetupFeeMinor: setupFeeMinor,
			RenewalAmountMinor: renewalAmountMinor,
		}
	}
	created, err := c.catalogWriter.CreatePriceSchedule(ctx, catalog.PriceScheduleCreateInput{
		VersionNo: req.VersionNo, BillingRuleNo: req.BillingRuleNo,
		ConfigurationPriceDimensionNo: req.ConfigurationPriceDimensionNo, CurrencyCode: req.CurrencyCode,
		Tiers: tiers, Active: req.Status == "active",
	})
	if err != nil {
		return nil, err
	}
	responseTiers := make([]*v1.CatalogPriceScheduleTierRes, len(created.Tiers))
	for index, tier := range created.Tiers {
		responseTiers[index] = &v1.CatalogPriceScheduleTierRes{
			PriceNo: created.PriceNumbers[index], MinimumQuantity: tier.MinimumQuantity,
			MaximumQuantity: tier.MaximumQuantity, AmountMinor: formatCatalogAmount(tier.AmountMinor),
			SetupFeeMinor: formatCatalogAmount(tier.SetupFeeMinor), RenewalAmountMinor: formatOptionalCatalogAmount(tier.RenewalAmountMinor),
		}
	}
	return &v1.CatalogPriceScheduleCreateRes{
		VersionNo: created.VersionNo, BillingRuleNo: created.BillingRuleNo,
		ConfigurationPriceDimensionNo: created.ConfigurationPriceDimensionNo, CurrencyCode: created.CurrencyCode,
		Tiers: responseTiers, Status: created.Status,
	}, nil
}
