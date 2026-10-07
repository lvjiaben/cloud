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
 ADD COLUMN IF NOT EXISTS resource_channel_ref VARCHAR(64) NOT NULL DEFAULT ''`); err != nil {
		t.Fatalf("install resource projection test columns: %v", err)
	}
}
