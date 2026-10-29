package productautomation

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

// installResourceProjectionTestColumns advances focused 074/075 fixtures to
// the interface-origin shape introduced by migration 096. The full 096
// migration and its constraints are exercised by the projection integration
// test; these service fixtures only need the current read columns.
func installResourceProjectionTestColumns(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_automation_interface_instance
 ADD COLUMN IF NOT EXISTS interface_origin_code VARCHAR(16) NOT NULL DEFAULT 'managed',
 ADD COLUMN IF NOT EXISTS resource_owner_id VARCHAR(64) NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS resource_channel_ref VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_catalog_product_version ADD COLUMN IF NOT EXISTS product_no TEXT DEFAULT '';

CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_import(
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,product_no TEXT,version_no TEXT,interface_instance_no TEXT,upstream_reference TEXT,source_snapshot JSONB);
`); err != nil {
		t.Fatalf("install resource projection test columns: %v", err)
	}
}
