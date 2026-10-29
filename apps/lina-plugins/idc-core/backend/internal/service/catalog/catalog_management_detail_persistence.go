// This file loads the bounded latest-version graph for one managed catalog product.

package catalog

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/idccap"
)

// Flat fields have distinct names as GoFrame recursively flattens anonymous
// embeddings before matching column names; duplicate Status/CreatedAt names
// would otherwise map a product value into its version.
type productManagementDetailHeader struct {
	CurrentVersionNo                                                        string
	DirectoryNo                                                             string
	ProductNo, CategoryNo, CategoryNameZhCN, CategoryNameEnUS               string
	ProductStatus, PublicSlug, NameZhCN, NameEnUS, SummaryZhCN, SummaryEnUS string
	SortOrder                                                               int
	DescriptionVersion                                                      int64
	ProductCreatedAt, ProductUpdatedAt                                      time.Time
	VersionNo                                                               string
	VersionNumber                                                           int
	VersionStatus                                                           string
	Source, SourceVersion, ChangeSummary                                    string

	PublishedAt, RetiredAt    *time.Time
	VersionCreatedAt          time.Time
	InventoryTrackingEnabled  bool
	InventorySellableQuantity int64
	InventoryVersion          int64
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
		Product:      ProductManagementDetailProduct{CurrentVersionNo: header.CurrentVersionNo, DirectoryNo: header.DirectoryNo, ProductNo: header.ProductNo, CategoryNo: header.CategoryNo, CategoryNameZhCN: header.CategoryNameZhCN, CategoryNameEnUS: header.CategoryNameEnUS, Status: header.ProductStatus, PublicSlug: header.PublicSlug, NameZhCN: header.NameZhCN, NameEnUS: header.NameEnUS, SummaryZhCN: header.SummaryZhCN, SummaryEnUS: header.SummaryEnUS, SortOrder: header.SortOrder, DescriptionVersion: header.DescriptionVersion, CreatedAt: header.ProductCreatedAt, UpdatedAt: header.ProductUpdatedAt},
		Version:      ProductManagementDetailVersion{VersionNo: header.VersionNo, VersionNumber: header.VersionNumber, Status: header.VersionStatus, Source: header.Source, SourceVersion: header.SourceVersion, ChangeSummary: header.ChangeSummary, PublishedAt: header.PublishedAt, RetiredAt: header.RetiredAt, CreatedAt: header.VersionCreatedAt},
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
SELECT p.current_version_no AS current_version_no, COALESCE(m.directory_no,'') AS directory_no, p.product_no, p.category_no, c.name_zh_cn AS category_name_zh_cn,
       c.name_en_us AS category_name_en_us, p.status_code AS product_status,
       p.public_slug, p.name_zh_cn, p.name_en_us, p.summary_zh_cn, p.summary_en_us,
       p.sort_order, p.version AS description_version, p.created_at AS product_created_at, p.updated_at AS product_updated_at,
       v.version_no, v.version_number, v.status_code AS version_status,
	       v.source_code AS source, v.source_version, v.change_summary,

	       v.published_at, v.retired_at,
	       v.created_at AS version_created_at,
	       inventory.tracking_enabled AS inventory_tracking_enabled,
	       inventory.sellable_quantity AS inventory_sellable_quantity,
	       inventory.version AS inventory_version
FROM plugin_idc_core_catalog_product p
LEFT JOIN plugin_idc_core_product_directory_membership m ON m.tenant_id=p.tenant_id AND m.product_no=p.product_no
JOIN plugin_idc_core_catalog_category c
  ON c.tenant_id = p.tenant_id AND c.category_no = p.category_no AND c.deleted_at IS NULL
JOIN LATERAL (
	    SELECT version_no, version_number, status_code, source_code, source_version,
	           change_summary,
	           published_at, retired_at, created_at
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
 dimension.pricing_mode_code AS configuration_pricing_mode,
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
