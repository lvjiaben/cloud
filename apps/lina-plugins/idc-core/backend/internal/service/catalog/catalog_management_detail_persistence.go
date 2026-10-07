// This file loads the bounded latest-version graph for one managed catalog product.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type productManagementDetailHeader struct {
	ProductManagementDetailProduct
	ProductManagementDetailVersion
	InventoryTrackingEnabled  bool  `orm:"inventory_tracking_enabled"`
	InventorySellableQuantity int64 `orm:"inventory_sellable_quantity"`
	InventoryVersion          int64 `orm:"inventory_version"`
}

const (
	maximumProductDetailBillingRules = 512
	maximumProductDetailPrices       = 4096
	maximumProductDetailTaxPolicies  = 64
)

func loadProductManagementDetail(ctx context.Context, productNo string) (*ProductManagementDetail, error) {
	header := productManagementDetailHeader{}
	if err := g.DB().GetScan(ctx, &header, productManagementDetailHeaderSQL,
		idccap.DefaultTenantID, productNo); err != nil {
		return nil, gerror.Wrap(err, "load catalog product management detail header")
	}
	if header.ProductNo == "" {
		return nil, nil
	}
	detail := &ProductManagementDetail{
		Product:      header.ProductManagementDetailProduct,
		Version:      header.ProductManagementDetailVersion,
		BillingRules: make([]ProductManagementDetailBillingRule, 0),
		Prices:       make([]ProductManagementDetailPrice, 0),
		TaxPolicies:  make([]ProductManagementDetailTaxPolicy, 0),
		Inventory: ProductVersionInventory{TrackingEnabled: header.InventoryTrackingEnabled,
			SellableQuantity: header.InventorySellableQuantity, Version: header.InventoryVersion},
	}
	if err := loadBoundedProductDetailRows(ctx, &detail.BillingRules, productManagementDetailBillingSQL,
		header.VersionNo, maximumProductDetailBillingRules); err != nil {
		return nil, err
	}
	if err := loadBoundedProductDetailRows(ctx, &detail.Prices, productManagementDetailPricesSQL,
		header.VersionNo, maximumProductDetailPrices); err != nil {
		return nil, err
	}
	if err := loadBoundedProductDetailRows(ctx, &detail.TaxPolicies, productManagementDetailTaxPoliciesSQL,
		header.VersionNo, maximumProductDetailTaxPolicies); err != nil {
		return nil, err
	}
	policy, err := loadFulfillmentPolicyEvidence(ctx, header.VersionNo)
	if err != nil {
		return nil, gerror.Wrap(err, "load catalog product fulfillment policy")
	}
	detail.FulfillmentPolicy = policy
	return detail, nil
}

func loadBoundedProductDetailRows[T any](
	ctx context.Context,
	target *[]T,
	query string,
	versionNo string,
	maximum int,
) error {
	if err := g.DB().GetScan(ctx, target, query,
		idccap.DefaultTenantID, versionNo, maximum+1); err != nil {
		return gerror.Wrap(err, "load catalog product management detail graph")
	}
	if len(*target) > maximum {
		return gerror.New("catalog product management detail collection exceeds bound")
	}
	return nil
}

const productManagementDetailHeaderSQL = `
SELECT p.product_no, p.category_no, c.name_zh_cn AS category_name_zh_cn,
       c.name_en_us AS category_name_en_us, p.status_code AS status,
       p.public_slug, p.name_zh_cn, p.name_en_us, p.summary_zh_cn, p.summary_en_us,
       p.sort_order, p.created_at, p.updated_at,
       v.version_no, v.version_number, v.status_code AS version_status,
	       v.source_code AS source, v.source_version, v.change_summary,
	       v.requires_contract, v.contract_policy_code, v.contract_template_no,
	       v.contract_policy_version, v.published_at, v.retired_at,
	       v.created_at AS version_created_at,
	       inventory.tracking_enabled AS inventory_tracking_enabled,
	       inventory.sellable_quantity AS inventory_sellable_quantity,
	       inventory.version AS inventory_version
FROM plugin_idc_core_catalog_product p
JOIN plugin_idc_core_catalog_category c
  ON c.tenant_id = p.tenant_id AND c.category_no = p.category_no AND c.deleted_at IS NULL
JOIN LATERAL (
	    SELECT version_no, version_number, status_code, source_code, source_version,
	           change_summary, requires_contract, contract_policy_code,
	           contract_template_no, contract_policy_version, published_at, retired_at, created_at
    FROM plugin_idc_core_catalog_product_version
    WHERE tenant_id = p.tenant_id AND product_no = p.product_no
    ORDER BY version_number DESC, id DESC LIMIT 1
) v ON TRUE
JOIN plugin_idc_core_catalog_product_version_inventory inventory ON inventory.version_no=v.version_no
WHERE p.tenant_id = ? AND p.product_no = ? AND p.deleted_at IS NULL`

const productManagementDetailBillingSQL = `SELECT billing_rule_no, rule_code,
 charge_type_code AS charge_type, period_unit_code AS period_unit, period_count,
 price_phase_code AS price_phase, quantity_mode_code AS quantity_mode, trial_quantity,
 status_code AS status, minimum_purchase_cycles, maximum_renewal_cycles, meter_unit_code AS meter_unit,
 tier_mode_code AS tier_mode, usage_collection_unit_code AS usage_collection_unit,
 usage_collection_count, usage_rounding_mode_code AS usage_rounding_mode,
 usage_rounding_quantity, usage_minimum_quantity, usage_maximum_quantity,
 usage_missing_policy_code AS usage_missing_policy
 FROM plugin_idc_core_catalog_billing_rule
 WHERE tenant_id = ? AND version_no = ? ORDER BY id LIMIT ?`

const productManagementDetailPricesSQL = `SELECT price.price_no, price.billing_rule_no,
 price.currency_code, price.amount_minor, price.setup_fee_minor, price.renewal_amount_minor, price.minimum_quantity,
 price.maximum_quantity, price.status_code AS status, price.configuration_price_dimension_no,
 dimension.field_label_zh_cn AS configuration_field_label_zh_cn,
 dimension.field_label_en_us AS configuration_field_label_en_us,
 dimension.candidate_label_zh_cn AS configuration_candidate_label_zh_cn,
 dimension.candidate_label_en_us AS configuration_candidate_label_en_us
 FROM plugin_idc_core_catalog_price price
 LEFT JOIN plugin_idc_core_catalog_configuration_price_dimension dimension
   ON dimension.version_no=price.version_no
  AND dimension.dimension_no=price.configuration_price_dimension_no
 WHERE price.tenant_id = ? AND price.version_no = ? AND price.option_no IS NULL
 ORDER BY price.billing_rule_no, price.configuration_price_dimension_no NULLS FIRST,
          price.currency_code, price.minimum_quantity, price.id LIMIT ?`

const productManagementDetailTaxPoliciesSQL = `SELECT tax_policy_no, currency_code,
 price_includes_tax, tax_treatment_code AS tax_treatment, tax_rate_basis_points,
 tax_category_code, rounding_scope_code AS rounding_scope,
 rounding_mode_code AS rounding_mode, rounding_quantum_minor
 FROM plugin_idc_core_catalog_tax_policy
 WHERE tenant_id = ? AND version_no = ? ORDER BY currency_code, id LIMIT ?`

const productManagementDetailFulfillmentPolicySQL = `SELECT strategy_code, granularity_code,
 ongoing_management, renewal_mode_code, renewal_method, entitlement_owner_id, provision_method, query_method,
 maximum_attempts, retry_base_seconds, policy_digest
 FROM plugin_idc_core_catalog_product_version_fulfillment_policy
 WHERE tenant_id = ? AND version_no = ? LIMIT 1`

func loadFulfillmentPolicyEvidence(ctx context.Context, versionNo string) (*FulfillmentPolicyEvidence, error) {
	var policy *FulfillmentPolicyEvidence
	if err := g.DB().GetScan(ctx, &policy, productManagementDetailFulfillmentPolicySQL,
		idccap.DefaultTenantID, versionNo); err != nil {
		return nil, err
	}
	return policy, nil
}
