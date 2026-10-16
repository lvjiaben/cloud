// This file loads bounded price facts for all currencies in five set queries.
// Legacy option identifiers and structured dimensions remain distinct so an
// importer cannot silently substitute a different configuration price source.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
)

func (s *publicCatalogReaderImpl) loadImportPriceSource(ctx context.Context, versionNo string) (*publicQuoteSource, []configurationPriceDimensionIdentity, error) {
	var (
		source     = &publicQuoteSource{}
		dimensions []configurationPriceDimensionIdentity
	)
	if err := s.database.GetScan(ctx, &source.BillingRules, importBillingRulesSQL, versionNo, maximumPublicationBillingRules+1); err != nil {
		return nil, nil, gerror.Wrap(err, "load import billing rules")
	}
	if len(source.BillingRules) == 0 || len(source.BillingRules) > maximumPublicationBillingRules {
		return nil, nil, gerror.New("import billing rule bound exceeded")
	}
	if err := s.database.GetScan(ctx, &source.Prices, importPricesSQL, versionNo, maximumPublicationPrices+1); err != nil {
		return nil, nil, gerror.Wrap(err, "load import prices")
	}
	if len(source.Prices) == 0 || len(source.Prices) > maximumPublicationPrices {
		return nil, nil, gerror.New("import price bound exceeded")
	}
	if err := s.database.GetScan(ctx, &source.TaxPolicies, importTaxSQL, versionNo, 65); err != nil {
		return nil, nil, gerror.Wrap(err, "load import tax policies")
	}
	if len(source.TaxPolicies) == 0 || len(source.TaxPolicies) > 64 {
		return nil, nil, gerror.New("import tax policy bound exceeded")
	}
	if err := s.database.GetScan(ctx, &dimensions, importDimensionsSQL, versionNo, maximumConfigurationPriceDimensions+1); err != nil {
		return nil, nil, gerror.Wrap(err, "load import price dimensions")
	}
	if len(dimensions) > maximumConfigurationPriceDimensions {
		return nil, nil, gerror.New("import price dimension bound exceeded")
	}
	if err := s.database.GetScan(ctx, &source.LegacyOptions, importLegacyOptionsSQL, versionNo, maximumPublicConfigurationOptions+1); err != nil {
		return nil, nil, gerror.Wrap(err, "load import legacy options")
	}
	if len(source.LegacyOptions) > maximumPublicConfigurationOptions {
		return nil, nil, gerror.New("import legacy option bound exceeded")
	}
	return source, dimensions, nil
}

const importPricesSQL = `SELECT version_no,billing_rule_no,option_no,configuration_price_dimension_no,
currency_code,amount_minor,setup_fee_minor,renewal_amount_minor,minimum_quantity,maximum_quantity
FROM plugin_idc_core_catalog_price
WHERE tenant_id=0 AND version_no=? AND status_code='active' ORDER BY id ASC LIMIT ?`

const importBillingRulesSQL = `SELECT version_no,billing_rule_no,charge_type_code,period_unit_code,period_count,
price_phase_code,quantity_mode_code,trial_quantity,minimum_purchase_cycles,maximum_renewal_cycles,
meter_unit_code,tier_mode_code,usage_collection_unit_code,usage_collection_count,
usage_rounding_mode_code,usage_rounding_quantity,usage_minimum_quantity,usage_maximum_quantity,usage_missing_policy_code
FROM plugin_idc_core_catalog_billing_rule WHERE tenant_id=0 AND version_no=? AND status_code='active' ORDER BY id ASC LIMIT ?`

const importTaxSQL = `SELECT currency_code,price_includes_tax,tax_treatment_code,tax_rate_basis_points,
tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor
FROM plugin_idc_core_catalog_tax_policy WHERE tenant_id=0 AND version_no=? ORDER BY id ASC LIMIT ?`

const importDimensionsSQL = `SELECT dimension_no,field_key,candidate_value,pricing_mode_code
FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active' ORDER BY field_key,candidate_value,id LIMIT ?`

const importLegacyOptionsSQL = `SELECT option.option_no,config_group.group_code,
       option.normalized_value::TEXT AS normalized_value
FROM plugin_idc_core_catalog_configuration_option option
INNER JOIN plugin_idc_core_catalog_configuration_group config_group
  ON config_group.tenant_id=option.tenant_id
 AND config_group.version_no=option.version_no
 AND config_group.group_no=option.group_no
WHERE option.tenant_id=0 AND option.version_no=? AND option.status_code='active'
ORDER BY config_group.sort_order,config_group.id,option.sort_order,option.id LIMIT ?`
