// This file validates the common sellability gate for every catalog source.

package catalog

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	maximumPublicationBillingRules = 256
	maximumPublicationPrices       = 100_000
	maximumPublicationTaxPolicies  = 64
)

type catalogPublicationGraph struct {
	billingRules       []entity.CatalogBillingRule
	prices             []entity.CatalogPrice
	taxPolicies        []entity.CatalogTaxPolicy
	priceDimensionNos  []string
	configurationModes map[string]ConfigurationPriceMode
}

// PublicationCheckCode is one stable, localized-by-the-client readiness dimension.
type PublicationCheckCode string

// Stable publication readiness dimensions shared by preview and commit validation.
const (
	PublicationCheckSalesCopy PublicationCheckCode = "sales_copy"
	PublicationCheckPricing   PublicationCheckCode = "pricing"
	PublicationCheckTax       PublicationCheckCode = "tax"
)

// PublicationReadinessCheck reports one bounded publication gate result.
type PublicationReadinessCheck struct {
	Code   PublicationCheckCode
	Passed bool
}

func validateCatalogPublicationCompleteness(
	ctx context.Context,
	product *entity.CatalogProduct,
	version *entity.CatalogProductVersion,
) error {
	checks, err := evaluateCatalogPublicationCompleteness(ctx, product, version)
	if err != nil {
		return err
	}
	for _, check := range checks {
		if !check.Passed {
			return publicationIncomplete()
		}
	}
	return nil
}

func evaluateCatalogPublicationCompleteness(
	ctx context.Context,
	product *entity.CatalogProduct,
	version *entity.CatalogProductVersion,
) ([]PublicationReadinessCheck, error) {
	checks := []PublicationReadinessCheck{
		{Code: PublicationCheckSalesCopy, Passed: product != nil && completePublicationProductCopy(product)},
		{Code: PublicationCheckPricing},
		{Code: PublicationCheckTax},
	}
	if product == nil || version == nil {
		return checks, nil
	}
	graph, err := loadCatalogPublicationGraph(ctx, version.VersionNo)
	if err != nil {
		if !bizerr.Is(err, CodePublicationIncomplete) {
			return nil, err
		}
		graph = nil
	}
	if graph != nil {
		checks[1].Passed = completePublicationPricing(graph)
		checks[2].Passed = completePublicationTaxPolicies(graph)
	}
	return checks, nil
}

func loadCatalogPublicationGraph(
	ctx context.Context,
	versionNo string,
) (*catalogPublicationGraph, error) {
	graph := &catalogPublicationGraph{}
	billingColumns := dao.CatalogBillingRule.Columns()
	if err := dao.CatalogBillingRule.Ctx(ctx).
		Fields(
			billingColumns.BillingRuleNo, billingColumns.ChargeTypeCode,
			billingColumns.PeriodUnitCode, billingColumns.PeriodCount,
			billingColumns.PricePhaseCode, billingColumns.QuantityModeCode,
			billingColumns.TrialQuantity, billingColumns.MinimumPurchaseCycles,
			billingColumns.MaximumRenewalCycles, billingColumns.MeterUnitCode,
			billingColumns.TierModeCode, billingColumns.UsageCollectionUnitCode,
			billingColumns.UsageCollectionCount, billingColumns.UsageRoundingModeCode,
			billingColumns.UsageRoundingQuantity, billingColumns.UsageMinimumQuantity,
			billingColumns.UsageMaximumQuantity, billingColumns.UsageMissingPolicyCode,
		).Where(do.CatalogBillingRule{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo, StatusCode: catalogStatusActive,
	}).OrderAsc(billingColumns.Id).Limit(maximumPublicationBillingRules + 1).
		Scan(&graph.billingRules); err != nil {
		return nil, gerror.Wrap(err, "load catalog publication billing rules")
	}
	if len(graph.billingRules) == 0 || len(graph.billingRules) > maximumPublicationBillingRules {
		return nil, publicationIncomplete()
	}
	if err := dao.CatalogPrice.Ctx(ctx).Raw(`SELECT billing_rule_no,
COALESCE(configuration_price_dimension_no,option_no) AS option_no,currency_code,
amount_minor,setup_fee_minor,renewal_amount_minor,minimum_quantity,maximum_quantity
FROM plugin_idc_core_catalog_price
WHERE tenant_id=0 AND version_no=? AND status_code='active'
  AND option_no IS NULL
ORDER BY id LIMIT ?`, versionNo, maximumPublicationPrices+1).Scan(&graph.prices); err != nil {
		return nil, gerror.Wrap(err, "load catalog publication prices")
	}
	if len(graph.prices) > maximumPublicationPrices {
		return nil, publicationIncomplete()
	}
	var dimensionRows []struct {
		DimensionNo string                 `orm:"dimension_no"`
		PricingMode ConfigurationPriceMode `orm:"pricing_mode_code"`
	}
	if err := dao.CatalogProductVersion.Ctx(ctx).Raw(`SELECT dimension_no,pricing_mode_code
FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active'
ORDER BY dimension_no LIMIT ?`, versionNo, maximumConfigurationPriceDimensions+1).Scan(&dimensionRows); err != nil {
		return nil, gerror.Wrap(err, "load catalog publication price dimensions")
	}
	if len(dimensionRows) > maximumConfigurationPriceDimensions {
		return nil, publicationIncomplete()
	}
	graph.configurationModes = make(map[string]ConfigurationPriceMode, len(dimensionRows))
	graph.priceDimensionNos = make([]string, 0, len(dimensionRows))
	for _, row := range dimensionRows {
		graph.configurationModes[row.DimensionNo] = row.PricingMode
		graph.priceDimensionNos = append(graph.priceDimensionNos, row.DimensionNo)
	}
	taxColumns := dao.CatalogTaxPolicy.Columns()
	if err := dao.CatalogTaxPolicy.Ctx(ctx).Fields(
		taxColumns.CurrencyCode, taxColumns.PriceIncludesTax,
		taxColumns.TaxTreatmentCode, taxColumns.TaxRateBasisPoints,
		taxColumns.TaxCategoryCode, taxColumns.RoundingScopeCode,
		taxColumns.RoundingModeCode, taxColumns.RoundingQuantumMinor,
	).Where(do.CatalogTaxPolicy{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
	}).OrderAsc(taxColumns.CurrencyCode).Limit(maximumPublicationTaxPolicies + 1).
		Scan(&graph.taxPolicies); err != nil {
		return nil, gerror.Wrap(err, "load catalog publication tax policies")
	}
	if len(graph.taxPolicies) > maximumPublicationTaxPolicies {
		return nil, publicationIncomplete()
	}
	return graph, nil
}

func completePublicationProductCopy(product *entity.CatalogProduct) bool {
	return strings.TrimSpace(product.NameZhCn) != "" && strings.TrimSpace(product.NameEnUs) != "" &&
		strings.TrimSpace(product.SummaryZhCn) != "" && strings.TrimSpace(product.SummaryEnUs) != ""
}

func completePublicationPricing(graph *catalogPublicationGraph) bool {
	index, err := NewCurrencyPricingIndex(graph.billingRules, graph.prices)
	if err != nil {
		return false
	}
	index.configurationModes = graph.configurationModes
	currencies := index.ConfiguredCurrencies()
	if len(currencies) == 0 {
		return false
	}
	for _, currencyCode := range currencies {
		result, evaluateErr := index.Evaluate(currencyCode, graph.priceDimensionNos)
		if evaluateErr != nil || !result.Sellable {
			return false
		}
	}
	return true
}

func completePublicationTaxPolicies(graph *catalogPublicationGraph) bool {
	if graph == nil {
		return false
	}
	pricedCurrencies := make(map[string]struct{})
	for _, price := range graph.prices {
		if !currencyCodePattern.MatchString(price.CurrencyCode) {
			return false
		}
		pricedCurrencies[price.CurrencyCode] = struct{}{}
	}
	if len(pricedCurrencies) == 0 || len(graph.taxPolicies) != len(pricedCurrencies) {
		return false
	}
	seen := make(map[string]struct{}, len(graph.taxPolicies))
	for _, policy := range graph.taxPolicies {
		if _, priced := pricedCurrencies[policy.CurrencyCode]; !priced {
			return false
		}
		if _, duplicate := seen[policy.CurrencyCode]; duplicate {
			return false
		}
		definition := TaxPolicyDefinition{
			PriceIncludesTax:     policy.PriceIncludesTax,
			TaxTreatment:         TaxTreatment(policy.TaxTreatmentCode),
			TaxRateBasisPoints:   policy.TaxRateBasisPoints,
			TaxCategoryCode:      policy.TaxCategoryCode,
			RoundingScope:        TaxRoundingScope(policy.RoundingScopeCode),
			RoundingMode:         TaxRoundingMode(policy.RoundingModeCode),
			RoundingQuantumMinor: policy.RoundingQuantumMinor,
		}
		if ValidateTaxPolicy(definition) != nil {
			return false
		}
		seen[policy.CurrencyCode] = struct{}{}
	}
	return true
}

func publicationIncomplete() error {
	return bizerr.NewCode(CodePublicationIncomplete)
}
