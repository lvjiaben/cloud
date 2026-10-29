// This file projects persisted priced selections and billing periods in one
// bounded owner-scoped query. It never resolves plugins or returns credentials.

package commerce

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// projectCartConfiguration joins only rows belonging to the already-owned cart.
func (s *serviceImpl) projectCartConfiguration(ctx context.Context, cart *Cart) error {
	var rows []struct {
		HasServerLogin                bool
		ServerLoginToken              string
		ServerLoginHostname           string
		ServerLoginPasswordConfigured bool
		CartItemNo                    string
		ChargeTypeCode                string
		PeriodUnitCode                string
		PeriodCount                   int
		LabelZhCN                     string `orm:"label_zh_cn"`
		LabelEnUS                     string `orm:"label_en_us"`
		ValueZhCN                     string `orm:"value_zh_cn"`
		ValueEnUS                     string `orm:"value_en_us"`
		PricingModeCode               catalog.ConfigurationPriceMode
		Quantity                      int
	}
	err := s.database.GetScan(ctx, &rows, `SELECT item.cart_item_no,(configuration.server_login_token <> '') AS has_server_login,configuration.server_login_token,COALESCE(login.hostname,'') AS server_login_hostname,(COALESCE(login.password_secret_ref,'') <> '') AS server_login_password_configured,rule.charge_type_code,rule.period_unit_code,rule.period_count,
COALESCE(dimension.field_label_zh_cn,'') AS label_zh_cn,COALESCE(dimension.field_label_en_us,'') AS label_en_us,
COALESCE(dimension.candidate_label_zh_cn,'') AS value_zh_cn,COALESCE(dimension.candidate_label_en_us,'') AS value_en_us,
COALESCE(dimension.pricing_mode_code,'') AS pricing_mode_code,COALESCE(selection.quantity,0) AS quantity
FROM plugin_idc_core_commerce_cart_item item
JOIN plugin_idc_core_commerce_configuration_snapshot configuration ON configuration.tenant_id=item.tenant_id AND configuration.configuration_snapshot_no=item.configuration_snapshot_no
LEFT JOIN plugin_idc_core_purchase_server_login login ON login.tenant_id=item.tenant_id AND login.server_login_token=configuration.server_login_token AND login.cart_item_no=item.cart_item_no
JOIN plugin_idc_core_catalog_billing_rule rule ON rule.tenant_id=item.tenant_id AND rule.billing_rule_no=item.billing_rule_no
LEFT JOIN plugin_idc_core_commerce_configuration_price_selection selection
 ON selection.tenant_id=item.tenant_id AND selection.configuration_snapshot_no=item.configuration_snapshot_no
LEFT JOIN plugin_idc_core_catalog_configuration_price_dimension dimension
 ON dimension.tenant_id=selection.tenant_id AND dimension.dimension_no=selection.dimension_no
WHERE item.tenant_id=0 AND item.cart_no=? AND item.status_code<>'removed'
ORDER BY item.sort_order,item.id,dimension.id LIMIT ?`, cart.CartNo, maximumCartItems*129+1)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(rows) > maximumCartItems*129 {
		return bizerr.NewCode(CodeUnavailable)
	}
	indexes := make(map[string]int, len(cart.Items))
	for index := range cart.Items {
		indexes[cart.Items[index].CartItemNo] = index
		cart.Items[index].Configuration = []CartConfiguration{}
	}
	for _, row := range rows {
		index, exists := indexes[row.CartItemNo]
		if !exists {
			return bizerr.NewCode(CodeUnavailable)
		}
		item := &cart.Items[index]
		item.HasServerLogin, item.ServerLoginToken, item.ServerLoginHostname, item.ServerLoginPasswordConfigured = row.HasServerLogin, row.ServerLoginToken, row.ServerLoginHostname, row.ServerLoginPasswordConfigured
		item.ChargeTypeCode, item.PeriodUnitCode, item.PeriodCount = row.ChargeTypeCode, row.PeriodUnitCode, row.PeriodCount
		if row.LabelZhCN == "" && row.LabelEnUS == "" {
			continue
		}
		valueZhCN, valueEnUS := row.ValueZhCN, row.ValueEnUS
		if row.PricingModeCode == catalog.ConfigurationPricePerUnit || row.PricingModeCode == catalog.ConfigurationPriceBandTotal || row.PricingModeCode == catalog.ConfigurationPriceGraduated {
			valueZhCN, valueEnUS = strconv.Itoa(row.Quantity), strconv.Itoa(row.Quantity)
		}
		item.Configuration = append(item.Configuration, CartConfiguration{LabelZhCN: row.LabelZhCN, LabelEnUS: row.LabelEnUS, ValueZhCN: valueZhCN, ValueEnUS: valueEnUS})
	}
	return nil
}
