// This file verifies query indexes required by generic commerce snapshots.
package commerce

import (
	"strings"
	"testing"
)

func TestGenericOrderItemQuantityIndexPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	definition, err := scenario.database.GetValue(scenario.ctx, `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = current_schema()
  AND tablename = 'plugin_idc_core_commerce_order_item_quantity_snapshot'
  AND indexname = 'idx_idc_core_order_item_quantity_order'`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(definition.String(), "(tenant_id, order_no, order_item_no)") {
		t.Fatalf("generic order item quantity index is missing or has unexpected definition: %s", definition.String())
	}
}
