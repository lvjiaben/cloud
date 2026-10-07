// This file verifies the PostgreSQL index contract for delivery status reconciliation.
package commerce

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestDeliveryReconciliationIntentIndexPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	var definition struct {
		Columns    string
		NonPartial bool
	}
	if err := database.GetScan(ctx, &definition, `
SELECT string_agg(attribute.attname, ',' ORDER BY key.ordinality) AS columns,
       index.indpred IS NULL AS non_partial
FROM pg_index index
JOIN pg_class index_relation ON index_relation.oid=index.indexrelid
JOIN pg_class table_relation ON table_relation.oid=index.indrelid
JOIN pg_namespace namespace ON namespace.oid=table_relation.relnamespace
CROSS JOIN LATERAL unnest(index.indkey) WITH ORDINALITY AS key(attnum, ordinality)
JOIN pg_attribute attribute
  ON attribute.attrelid=table_relation.oid AND attribute.attnum=key.attnum
WHERE namespace.nspname=current_schema()
  AND table_relation.relname='plugin_idc_core_product_fulfillment_intent'
  AND index_relation.relname='idx_idc_product_fulfillment_intent_order_item_status'
GROUP BY index.indpred`); err != nil {
		t.Fatalf("read delivery reconciliation index definition: %v", err)
	}
	if definition.Columns != "tenant_id,order_item_no,status_code" || !definition.NonPartial {
		t.Fatalf("unexpected delivery reconciliation index definition: %+v", definition)
	}

	var plan string
	err := database.Transaction(ctx, func(transactionContext context.Context, _ gdb.TX) error {
		if _, execErr := database.Exec(transactionContext, `SET LOCAL enable_seqscan=off`); execErr != nil {
			return execErr
		}
		value, explainErr := database.GetValue(transactionContext, `
EXPLAIN (FORMAT JSON, COSTS OFF)
SELECT COUNT(*) AS intent_count,
       COUNT(*) FILTER (WHERE status_code='completed') AS completed_count,
       COUNT(*) FILTER (WHERE status_code IN ('completed','incomplete','canceled')) AS terminal_count,
       COUNT(*) FILTER (WHERE status_code='incomplete') AS incomplete_count,
       COUNT(*) FILTER (WHERE status_code='canceled') AS canceled_count
FROM plugin_idc_core_product_fulfillment_intent
WHERE tenant_id=0 AND order_item_no='order-item-index-contract'`)
		if explainErr != nil {
			return explainErr
		}
		plan = value.String()
		return nil
	})
	if err != nil {
		t.Fatalf("explain delivery reconciliation query: %v", err)
	}
	if !strings.Contains(plan, `"Index Name":`) ||
		!strings.Contains(plan, `"Index Cond":`) ||
		!strings.Contains(plan, "tenant_id") ||
		!strings.Contains(plan, "order_item_no") {
		t.Fatalf("delivery reconciliation query did not use a suitable tenant and order-item index: %s", plan)
	}
}
