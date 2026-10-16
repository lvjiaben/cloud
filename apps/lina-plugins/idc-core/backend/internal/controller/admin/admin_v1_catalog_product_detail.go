// This file maps the catalog product detail service graph to the admin API contract.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductDetail(ctx context.Context, req *v1.CatalogProductDetailReq) (res *v1.CatalogProductDetailRes, err error) {
	detail, err := c.catalogManagement.GetProductDetail(ctx, req.ProductNo)
	if err != nil {
		return nil, err
	}
	res = toAPICatalogProductDetail(detail)
	binding, bindingErr := c.productAutomation.GetBinding(ctx, detail.Version.VersionNo)
	if bindingErr != nil {
		return nil, bindingErr
	}
	res.FulfillmentBinding = mapAutomationBinding(binding)
	var contract *catalog.FrozenFulfillmentContract
	if detail.FulfillmentPolicy != nil && detail.FulfillmentPolicy.StrategyCode != catalog.FulfillmentStrategyManual {
		contract, bindingErr = c.productAutomation.GetFrozenFulfillmentContract(ctx, detail.Version.VersionNo)
		if bindingErr != nil {
			return nil, bindingErr
		}
	}
	if detail.FulfillmentPolicy != nil {
		if bindingErr = catalog.ValidateFulfillmentPolicyEvidence(*detail.FulfillmentPolicy, contract); bindingErr != nil {
			return nil, bindingErr
		}
	}
	res.FulfillmentPolicy = mapFulfillmentPolicySummary(detail.FulfillmentPolicy, contract)
	return res, nil
}

func mapFulfillmentPolicySummary(
	evidence *catalog.FulfillmentPolicyEvidence,
	contract *catalog.FrozenFulfillmentContract,
) *v1.CatalogFulfillmentPolicySummary {
	if evidence == nil {
		return nil
	}
	result := &v1.CatalogFulfillmentPolicySummary{StrategyCode: evidence.StrategyCode}
	result.TrialSupported = evidence.StrategyCode != catalog.FulfillmentStrategyManual && contract != nil && contract.TrialPauseResume
	if evidence.StrategyCode != catalog.FulfillmentStrategyManual && contract != nil && contract.Capability != nil {
		result.CapabilityZhCn = contract.Capability.ZhCN
		result.CapabilityEnUs = contract.Capability.EnUS
	}
	switch evidence.GranularityCode {
	case productfulfillmentcap.GranularityPerOrderItem:
		result.QuantityHandling = "together"
	case productfulfillmentcap.GranularityPerQuantityUnit:
		result.QuantityHandling = "separately"
	}
	return result
}

func toAPICatalogProductDetail(detail *catalog.ProductManagementDetail) *v1.CatalogProductDetailRes {
	product := detail.Product
	version := detail.Version
	res := &v1.CatalogProductDetailRes{
		Product: &v1.CatalogProductDetailProduct{
			DirectoryNo: product.DirectoryNo, DescriptionVersion: product.DescriptionVersion,
			ProductNo: product.ProductNo, CategoryNo: product.CategoryNo,
			CategoryNameZhCn: product.CategoryNameZhCN, CategoryNameEnUs: product.CategoryNameEnUS,
			Status: product.Status, PublicSlug: product.PublicSlug, NameZhCn: product.NameZhCN,
			NameEnUs: product.NameEnUS, SummaryZhCn: product.SummaryZhCN, SummaryEnUs: product.SummaryEnUS,
			SortOrder: product.SortOrder, CreatedAt: product.CreatedAt.UnixMilli(), UpdatedAt: product.UpdatedAt.UnixMilli(),
		},
		Version: &v1.CatalogProductDetailVersion{
			VersionNo: version.VersionNo, VersionNumber: version.VersionNumber, Status: version.Status,
			Source: version.Source, SourceVersion: version.SourceVersion, ChangeSummary: version.ChangeSummary,
			RequiresContract: version.RequiresContract, ContractPolicyCode: version.ContractPolicyCode,
			ContractTemplateNo: version.ContractTemplateNo, ContractPolicyVersion: version.ContractPolicyVersion,
			PublishedAt: toUnixMilliseconds(version.PublishedAt),
			RetiredAt:   toUnixMilliseconds(version.RetiredAt), CreatedAt: version.CreatedAt.UnixMilli(),
		},
		BillingRules: make([]*v1.CatalogProductDetailBillingRule, 0, len(detail.BillingRules)),
		Prices:       make([]*v1.CatalogProductDetailPrice, 0, len(detail.Prices)),
		TaxPolicies:  make([]*v1.CatalogProductDetailTaxPolicy, 0, len(detail.TaxPolicies)),
		Inventory: &v1.CatalogProductVersionInventory{TrackingEnabled: detail.Inventory.TrackingEnabled,
			SellableQuantity: detail.Inventory.SellableQuantity, Version: detail.Inventory.Version},
	}
	for _, item := range detail.BillingRules {
		res.BillingRules = append(res.BillingRules, &v1.CatalogProductDetailBillingRule{
			BillingRuleNo: item.BillingRuleNo, RuleCode: item.RuleCode, ChargeType: item.ChargeType,
			PeriodUnit: item.PeriodUnit, PeriodCount: item.PeriodCount, PricePhase: item.PricePhase,
			QuantityMode: item.QuantityMode, TrialQuantity: item.TrialQuantity, Status: item.Status,
			MinimumPurchaseCycles: item.MinimumPurchaseCycles, MaximumRenewalCycles: item.MaximumRenewalCycles,
			MeterUnit: item.MeterUnit, TierMode: item.TierMode, UsageCollectionUnit: item.UsageCollectionUnit,
			UsageCollectionCount: item.UsageCollectionCount, UsageRoundingMode: item.UsageRoundingMode,
			UsageRoundingQuantity: item.UsageRoundingQuantity, UsageMinimumQuantity: item.UsageMinimumQuantity,
			UsageMaximumQuantity: item.UsageMaximumQuantity, UsageMissingPolicy: item.UsageMissingPolicy,
		})
	}
	for _, item := range detail.Prices {
		res.Prices = append(res.Prices, &v1.CatalogProductDetailPrice{
			PriceNo: item.PriceNo, BillingRuleNo: item.BillingRuleNo,
			CurrencyCode: item.CurrencyCode, AmountMinor: formatCatalogAmount(item.AmountMinor),
			SetupFeeMinor:      formatCatalogAmount(item.SetupFeeMinor),
			RenewalAmountMinor: formatOptionalCatalogAmount(item.RenewalAmountMinor), MinimumQuantity: item.MinimumQuantity,
			MaximumQuantity: item.MaximumQuantity, Status: item.Status,
			ConfigurationPriceDimensionNo:   item.ConfigurationPriceDimensionNo,
			ConfigurationFieldLabelZhCn:     item.ConfigurationFieldLabelZhCN,
			ConfigurationFieldLabelEnUs:     item.ConfigurationFieldLabelEnUS,
			ConfigurationCandidateLabelZhCn: item.ConfigurationCandidateLabelZhCN,
			ConfigurationCandidateLabelEnUs: item.ConfigurationCandidateLabelEnUS,
		})
	}
	for _, item := range detail.TaxPolicies {
		res.TaxPolicies = append(res.TaxPolicies, &v1.CatalogProductDetailTaxPolicy{
			TaxPolicyNo: item.TaxPolicyNo, CurrencyCode: item.CurrencyCode,
			PriceIncludesTax: item.PriceIncludesTax, TaxTreatment: item.TaxTreatment,
			TaxRateBasisPoints: item.TaxRateBasisPoints, TaxCategoryCode: item.TaxCategoryCode,
			RoundingScope: item.RoundingScope, RoundingMode: item.RoundingMode,
			RoundingQuantumMinor: formatCatalogAmount(item.RoundingQuantumMinor),
		})
	}
	return res
}
