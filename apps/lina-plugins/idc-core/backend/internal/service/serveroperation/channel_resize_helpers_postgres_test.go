// The shared fixed-option snapshot writer must preserve production version FKs
// and option counts even without a generic core price-dimension mapping.

package serveroperation

import (
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"strings"
	"testing"
)

func TestSharedResizeConfigurationUsesRealVersionAndItemCountsPostgreSQL(t *testing.T) {
	f := newAccessoryCheckoutFixture(t)
	nativeLifecycleMigration(t, f.database, "094-generic-configuration-pricing.sql")
	if _, err := f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_catalog_configuration_group(group_no,version_no,group_code,selection_mode_code,name_zh_cn,name_en_us) VALUES('resize-group','ver_checkout_test','cpu','single','CPU','CPU');
 INSERT INTO plugin_idc_core_catalog_configuration_option(option_no,group_no,version_no,option_code,component_type_code,name_zh_cn,name_en_us,normalized_value,maximum_quantity) VALUES('resize-option','resize-group','ver_checkout_test','cpu2','cpu','2核','2 cores','{"cores":2}',1)`); err != nil {
		t.Fatal(err)
	}
	resolved := &catalog.ServerResizeConfiguration{ProductNo: "prd_checkout_test", VersionNo: "ver_checkout_test", SourceRevisionDigest: strings.Repeat("b", 64), Items: []catalog.PurchaseConfigurationItem{{GroupNo: "resize-group", OptionNo: "resize-option", GroupCode: "cpu", OptionCode: "cpu2", ComponentTypeCode: "cpu", GroupNameZhCN: "CPU", GroupNameEnUS: "CPU", OptionNameZhCN: "2核", OptionNameEnUS: "2 cores", NormalizedValue: `{"cores":2}`, Quantity: 1}}}
	if err := PersistChannelResizeConfiguration(f.ctx, f.database, resolved, "resize-target"); err != nil {
		t.Fatal(err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_configuration_snapshot_item WHERE configuration_snapshot_no='resize-target' AND version_no='ver_checkout_test' AND option_no='resize-option'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_configuration_snapshot WHERE configuration_snapshot_no='resize-target' AND selected_option_count=1 AND total_selected_quantity=1`, 1)
}
