// This file delegates billing-rule appends to the transactional catalog service.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogBillingRuleCreate(
	ctx context.Context,
	req *v1.CatalogBillingRuleCreateReq,
) (*v1.CatalogBillingRuleCreateRes, error) {
	return saveCatalogSettings(ctx, c.catalogPublication, req.VersionNo, func(ctx context.Context) (*v1.CatalogBillingRuleCreateRes, error) {
		created, err := c.catalogWriter.CreateBillingRule(ctx, catalog.BillingRuleCreateInput{
			VersionNo: req.VersionNo, RuleCode: req.RuleCode, Active: req.Status == "active",
			Definition: catalog.BillingRuleDefinition{
				ChargeType: catalog.ChargeType(req.ChargeType), PeriodUnit: catalog.PeriodUnit(req.PeriodUnit),
				PeriodCount: req.PeriodCount, PricePhase: catalog.PricePhase(req.PricePhase),
				QuantityMode: catalog.QuantityMode(req.QuantityMode), TrialQuantity: req.TrialQuantity,
				MinimumPurchaseCycles: req.MinimumPurchaseCycles,
				MaximumRenewalCycles:  req.MaximumRenewalCycles, MeterUnit: req.MeterUnit,
				TierMode: req.TierMode, UsageCollectionUnit: catalog.PeriodUnit(req.UsageCollectionUnit),
				UsageCollectionCount:   req.UsageCollectionCount,
				UsageRoundingMode:      catalog.RoundingMode(req.UsageRoundingMode),
				UsageRoundingQuantity:  req.UsageRoundingQuantity,
				UsageMinimumQuantity:   req.UsageMinimumQuantity,
				UsageMaximumQuantity:   req.UsageMaximumQuantity,
				UsageMissingDataPolicy: catalog.MissingUsagePolicy(req.UsageMissingDataPolicy),
			},
		})
		if err != nil {
			return nil, err
		}
		return &v1.CatalogBillingRuleCreateRes{
			BillingRuleNo: created.BillingRuleNo, VersionNo: created.VersionNo,
			RuleCode: created.RuleCode, ChargeType: string(created.Definition.ChargeType),
			PeriodUnit: string(created.Definition.PeriodUnit), PeriodCount: created.Definition.PeriodCount,
			PricePhase:   string(created.Definition.PricePhase),
			QuantityMode: string(created.Definition.QuantityMode), Status: created.Status,
		}, nil
	})
}
