// This file supplies a bounded product-owner projection for traffic-package
// configuration. Callers must authorize installation-level accessory settings;
// no catalog entities or customer information cross the component boundary.

package catalog

import (
	"context"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

// TrafficPackageProduct is a name-only choice, never a purchasable product quote.
type TrafficPackageProduct struct {
	ProductNo string `json:"productNo" orm:"product_no"`
	NameZhCN  string `json:"nameZhCN" orm:"name_zh_cn"`
	NameEnUS  string `json:"nameEnUS" orm:"name_en_us"`
}

// TrafficPackageProducts resolves at most 100 exact IDs or 50 name matches in
// one query. The exact-ID mode also validates an entire submitted scope at once.
// Current-version policy and binding are the catalog owner's frozen evidence
// of a managed server product; names, legacy rows and plugin names are not.
func (s *publicCatalogReaderImpl) TrafficPackageProducts(ctx context.Context, keyword string, numbers []string) ([]TrafficPackageProduct, error) {
	keyword = strings.TrimSpace(keyword)
	if len(numbers) > 100 || utf8.RuneCountInString(keyword) > 120 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	query := s.database.Model("plugin_idc_core_catalog_product product").Ctx(ctx).
		InnerJoin("plugin_idc_core_catalog_product_version version", "version.tenant_id=product.tenant_id AND version.product_no=product.product_no AND version.version_no=product.current_version_no").
		InnerJoin("plugin_idc_core_catalog_product_version_fulfillment_binding binding", "binding.tenant_id=version.tenant_id AND binding.version_no=version.version_no").
		InnerJoin("plugin_idc_core_catalog_product_version_fulfillment_policy policy", "policy.tenant_id=version.tenant_id AND policy.version_no=version.version_no").
		Fields("product.product_no,product.name_zh_cn,product.name_en_us").Where("product.tenant_id", 0).
		WhereNot("policy.strategy_code", FulfillmentStrategyManual).Where("policy.ongoing_management", true).
		Where("policy.entitlement_owner_id", provisionresourcecap.OwnerID)
	if len(numbers) > 0 {
		query = query.WhereIn("product.product_no", numbers).Limit(100)
	} else {
		pattern := "%" + escapeCatalogLikePrefix(keyword)
		query = query.Where("(product.name_zh_cn ILIKE ? OR product.name_en_us ILIKE ?)", pattern, pattern).Limit(50)
	}
	rows := []TrafficPackageProduct{}
	if err := query.OrderAsc("product.product_no").Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return rows, nil
}
