// Native accessory renewals obtain tax facts from the parent's existing frozen
// commercial evidence instead of manufacturing a legacy price or server record.

package serveroperation

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
)

// loadNativeAccessoryRenewalPrice freezes the currently published accessory
// amount and period, retaining the parent's effective immutable tax treatment.
func loadNativeAccessoryRenewalPrice(ctx context.Context, database gdb.DB, row *dueAccessoryRenewal) (*accessoryRenewalPrice, error) {
	var price *accessoryRenewalPrice
	if err := database.GetScan(ctx, &price, `SELECT policy.policy_no,version.policy_version,
version.currency_code,version.unit_amount_minor,version.period_unit_code,version.period_count,
version.content_digest,kind.name_zh_cn,kind.name_en_us,
COALESCE(current_price.tax_treatment_code,item.tax_treatment_code) AS tax_treatment_code,
CASE WHEN current_price.price_snapshot_no IS NOT NULL THEN current_price.tax_rate_basis_points
 ELSE item.tax_rate_basis_points END AS tax_rate_basis_points,
COALESCE(current_price.tax_category_code,item.tax_category_code) AS tax_category_code,
COALESCE(current_price.rounding_scope_code,item.rounding_scope_code) AS rounding_scope_code,
COALESCE(current_price.rounding_mode_code,item.rounding_mode_code) AS rounding_mode_code,
COALESCE(current_price.rounding_quantum_minor,item.rounding_quantum_minor) AS rounding_quantum_minor
FROM plugin_idc_core_server_accessory_price_policy policy
JOIN plugin_idc_core_server_accessory_price_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
JOIN plugin_idc_core_server_accessory_kind kind ON kind.accessory_kind=version.accessory_kind
JOIN plugin_idc_core_service_entitlement owner
 ON owner.tenant_id=policy.tenant_id AND owner.entitlement_no=? AND owner.customer_no=?
 AND owner.currency_code=? AND owner.entitlement_owner_id='idc-resource'
JOIN plugin_idc_core_commerce_order_item item
 ON item.tenant_id=owner.tenant_id AND item.order_item_no=owner.order_item_no AND item.order_no=owner.order_no
LEFT JOIN plugin_idc_core_service_price_snapshot current_price
 ON current_price.tenant_id=owner.tenant_id AND current_price.price_snapshot_no=owner.current_price_snapshot_no
 AND current_price.subject_type_code='entitlement' AND current_price.subject_no=owner.entitlement_no
WHERE policy.tenant_id=0 AND policy.accessory_kind=? AND policy.currency_code=?`,
		row.SubjectNo, row.CustomerNo, row.CurrencyCode, row.AccessoryKind, row.CurrencyCode); err != nil {
		return nil, err
	}
	if price == nil {
		return nil, ErrServiceUnavailable
	}
	return price, nil
}

// nullableAccessoryServiceNo keeps native identities out of the legacy service column.
func nullableAccessoryServiceNo(number string) any {
	if number == "" {
		return nil
	}
	return number
}
