// This file verifies timeline compaction preserves recovery boundaries and
// authoritative attempt history using transaction-local tables only.
package productfulfillment

import (
	"context"
	"os"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestProcessingTimelineCompactsBeforeLimitPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		_, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_product_fulfillment_command(tenant_id INT,intent_no TEXT,action_code TEXT,reason_code TEXT,reason_label_zh_cn TEXT,reason_label_en_us TEXT,safe_note TEXT,actor_user_id INT,created_at TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_system_transition(tenant_id INT,intent_no TEXT,event_code TEXT,reason_code TEXT,reason_label_zh_cn TEXT,reason_label_en_us TEXT,created_at TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(tenant_id INT,intent_no TEXT,task_no TEXT,created_at TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_attempt(task_no TEXT,result_status_code TEXT,finished_at TIMESTAMPTZ) ON COMMIT DROP;
INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'test-intent','test-task','2026-10-22 00:00:00Z');
INSERT INTO plugin_idc_core_product_fulfillment_attempt SELECT 'test-task','running','2026-10-22 00:00:00Z'::timestamptz + n*INTERVAL '1 second' FROM generate_series(1,100) n;
INSERT INTO plugin_idc_core_product_fulfillment_attempt VALUES('test-task','failed','2026-10-22 00:01:41Z');
INSERT INTO plugin_idc_core_product_fulfillment_command VALUES(0,'test-intent','recover_preparation','preparation_resolved','开通准备问题已处理','Provisioning prerequisites repaired','keep recovery boundary',0,'2026-10-22 00:01:42Z');
INSERT INTO plugin_idc_core_product_fulfillment_attempt SELECT 'test-task','running','2026-10-22 00:01:42Z'::timestamptz + n*INTERVAL '1 second' FROM generate_series(1,100) n;
INSERT INTO plugin_idc_core_product_fulfillment_attempt VALUES('test-task','succeeded','2026-10-22 00:03:23Z');
INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'other-intent','other-task','2026-10-22 00:00:00Z');
INSERT INTO plugin_idc_core_product_fulfillment_attempt VALUES('other-task','failed','2026-10-22 00:03:24Z');`)
		if err != nil {
			return err
		}
		service := &serviceImpl{database: db}
		events, err := service.loadSafeEvents(ctx, &intentDetailRow{IntentNo: "test-intent", InitialStatusCode: "queued", FinanciallySettledAt: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			return err
		}
		want := []string{"intent_created", "automation_started", "automation_processing", "automation_failed", "preparation_recovered", "automation_processing", "automation_completed"}
		if len(events) != len(want) {
			t.Fatalf("timeline dropped stages or retained repeated polls: %+v", events)
		}
		for i, event := range events {
			if event.TypeCode != want[i] {
				t.Fatalf("stage %d=%s want %s", i, event.TypeCode, want[i])
			}
		}
		count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_attempt WHERE task_no='test-task'`)
		if err != nil {
			return err
		}
		if count.Int() != 202 {
			t.Fatal("timeline compaction changed authoritative attempts")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
