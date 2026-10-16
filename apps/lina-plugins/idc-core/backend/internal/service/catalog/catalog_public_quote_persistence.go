// This file loads one currency's published pricing and tax facts with fixed query counts.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

type publicQuoteSource struct {
	BillingRules       []entity.CatalogBillingRule
	Prices             []entity.CatalogPrice
	TaxPolicies        []entity.CatalogTaxPolicy
	ConfigurationModes map[string]ConfigurationPriceMode
	// LegacyOptions resolves legacy option_no price dimensions to schema-safe
	// field/value pairs during portable export.
	LegacyOptions []importLegacyOptionMapping
}

func loadPublicQuoteSource(
	ctx context.Context,
	database gdb.DB,
	versionNo string,
	currency string,
) (*publicQuoteSource, error) {
	source := &publicQuoteSource{}
	if err := database.GetScan(ctx, &source.BillingRules, publicQuoteBillingRulesSQL,
		versionNo, maximumPublicationBillingRules+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load public quote billing rules")
	}
	if len(source.BillingRules) == 0 || len(source.BillingRules) > maximumPublicationBillingRules {
		return nil, gerror.New("public quote billing rule bound exceeded")
	}
	if err := database.GetScan(ctx, &source.Prices, publicQuotePricesSQL,
		versionNo, currency, maximumPublicationPrices+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load public quote prices")
	}
	if len(source.Prices) > maximumPublicationPrices {
		return nil, gerror.New("public quote price bound exceeded")
	}
	if err := database.GetScan(ctx, &source.TaxPolicies, publicQuoteTaxPolicySQL,
		versionNo, currency,
	); err != nil {
		return nil, gerror.Wrap(err, "load public quote tax policy")
	}
	var dimensions []configurationPriceDimensionIdentity
	if err := database.GetScan(ctx, &dimensions, `SELECT dimension_no,pricing_mode_code FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active' LIMIT ?`, versionNo, maximumConfigurationPriceDimensions+1); err != nil {
		return nil, gerror.Wrap(err, "load configuration pricing algorithms")
	}
	if len(dimensions) > maximumConfigurationPriceDimensions {
		return nil, gerror.New("configuration pricing dimension bound exceeded")
	}
	source.ConfigurationModes = make(map[string]ConfigurationPriceMode, len(dimensions))
	for _, dimension := range dimensions {
		source.ConfigurationModes[dimension.DimensionNo] = dimension.PricingMode
	}
	return source, nil
}

const publicQuoteBillingRulesSQL = `
SELECT billing_rule_no, charge_type_code, period_unit_code, period_count,
       price_phase_code, quantity_mode_code, trial_quantity,
       minimum_purchase_cycles, maximum_renewal_cycles, meter_unit_code,
       tier_mode_code, usage_collection_unit_code, usage_collection_count,
       usage_rounding_mode_code, usage_rounding_quantity,
       usage_minimum_quantity, usage_maximum_quantity, usage_missing_policy_code
FROM plugin_idc_core_catalog_billing_rule
WHERE tenant_id = 0 AND version_no = ? AND status_code = 'active'
ORDER BY id ASC
LIMIT ?`

const publicQuotePricesSQL = `
SELECT billing_rule_no, COALESCE(configuration_price_dimension_no,option_no) AS option_no,
       currency_code, amount_minor, setup_fee_minor,
       renewal_amount_minor, minimum_quantity, maximum_quantity
FROM plugin_idc_core_catalog_price
WHERE tenant_id = 0 AND version_no = ? AND currency_code = ? AND status_code = 'active'
ORDER BY id ASC
LIMIT ?`

const publicQuoteTaxPolicySQL = `
SELECT currency_code, price_includes_tax, tax_treatment_code, tax_rate_basis_points,
       tax_category_code, rounding_scope_code, rounding_mode_code, rounding_quantum_minor
FROM plugin_idc_core_catalog_tax_policy
WHERE tenant_id = 0 AND version_no = ? AND currency_code = ?
ORDER BY id ASC
LIMIT 2`
