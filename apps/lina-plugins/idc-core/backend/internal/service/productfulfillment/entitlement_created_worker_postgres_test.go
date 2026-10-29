// These tests verify local resource report dependencies do not consume binding
// attempts or cause a thirty-second retry, using transaction-local tables only.

package productfulfillment

import (
	"context"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

type entitlementDependencySink struct {
	ready bool
	calls int
}

func (s *entitlementDependencySink) Deliver(_ context.Context, event entitlementcreatedcap.Event) (*entitlementcreatedcap.Receipt, error) {
	s.calls++
	if !s.ready {
		return nil, gerror.New("local resource receipt not committed yet")
	}
	return &entitlementcreatedcap.Receipt{Version: entitlementcreatedcap.ContractVersion, EventNo: event.EventNo, OwnerReference: "mapping-owned", EvidenceDigest: strings.Repeat("a", 64)}, nil
}

type entitlementDependencyRouter struct{ sink *entitlementDependencySink }

func (r entitlementDependencyRouter) Resolve(string) (entitlementcreatedcap.Sink, bool) {
	return r.sink, true
}

func TestEntitlementBindingWaitsForExactLocalOutcomePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	for _, scenario := range []struct {
		name, owner, taskStatus, reportStatus, reportProvider string
		entitlementStatus                                     string
		blocked                                               bool
	}{
		{name: "pending-activation-before-task", owner: provisionresourcecap.OwnerID, taskStatus: "pending_query", reportStatus: "pending", reportProvider: "resource-current", entitlementStatus: "pending_activation", blocked: true},
		{name: "failed-but-recoverable-entitlement", owner: provisionresourcecap.OwnerID, taskStatus: "failed", reportStatus: "reported", reportProvider: "resource-current", entitlementStatus: "pending_activation", blocked: true},
		{name: "pending-local-report", owner: provisionresourcecap.OwnerID, taskStatus: "succeeded", reportStatus: "pending", reportProvider: "resource-current", blocked: true},
		{name: "retry-local-report", owner: provisionresourcecap.OwnerID, taskStatus: "succeeded", reportStatus: "retry_wait", reportProvider: "resource-current", blocked: true},
		{name: "inflight-local-report", owner: provisionresourcecap.OwnerID, taskStatus: "succeeded", reportStatus: "delivering", reportProvider: "resource-current", blocked: true},
		{name: "reported-local-outcome", owner: provisionresourcecap.OwnerID, taskStatus: "succeeded", reportStatus: "reported", reportProvider: "resource-current"},
		{name: "old-rejected-task-report", owner: provisionresourcecap.OwnerID, taskStatus: "succeeded", reportStatus: "pending", reportProvider: "resource-old"},
		{name: "other-business-owner", owner: "idc-peer", taskStatus: "succeeded", reportStatus: "pending", reportProvider: "resource-current"},
		{name: "manual-report-keeps-original-bounds", owner: provisionresourcecap.OwnerID, taskStatus: "succeeded", reportStatus: "manual_attention", reportProvider: "resource-current"},
		{name: "failed-task-keeps-original-bounds", owner: provisionresourcecap.OwnerID, taskStatus: "failed", reportStatus: "pending", reportProvider: "resource-current"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := database.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
				if _, err := database.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_entitlement_created_outbox(
 id BIGSERIAL,event_no TEXT,entitlement_no TEXT,intent_no TEXT,customer_no TEXT,product_no TEXT,version_no TEXT,
 product_name_zh_cn TEXT,product_name_en_us TEXT,configuration_snapshot_no TEXT,interface_instance_no TEXT,
 plugin_id TEXT,plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,owner_id TEXT,
 tenant_id INT DEFAULT 0,status_code TEXT DEFAULT 'pending',attempt_count INT DEFAULT 0,
 next_attempt_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,lease_token TEXT DEFAULT '',lease_expires_at TIMESTAMPTZ,
 last_error_code TEXT DEFAULT '',owner_reference TEXT,receipt_evidence_digest TEXT,updated_at TIMESTAMPTZ) ON COMMIT DROP;
 CREATE TEMP TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT,source_intent_no TEXT,customer_no TEXT,status_code TEXT) ON COMMIT DROP;
 CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(tenant_id INT,task_no TEXT,intent_no TEXT,status_code TEXT,external_reference TEXT) ON COMMIT DROP;
 CREATE TEMP TABLE plugin_idc_core_product_fulfillment_resource_outcome_outbox(tenant_id INT,task_no TEXT,provider_task_no TEXT,status_code TEXT) ON COMMIT DROP;`); err != nil {
					return err
				}
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_entitlement_created_outbox(event_no,entitlement_no,intent_no,customer_no,product_no,version_no,product_name_zh_cn,product_name_en_us,configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,owner_id) VALUES('event-owned','ent-owned','intent-owned','customer-owned','product-owned','version-owned','云服务器','Cloud server','snapshot-owned','interface-owned','connector-owned','v1.0.0','v1',?,?)`, strings.Repeat("b", 64), scenario.owner); err != nil {
					return err
				}
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'core-task-owned','intent-owned',?,'resource-current')`, scenario.taskStatus); err != nil {
					return err
				}
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_resource_outcome_outbox VALUES(0,'core-task-owned',?,?)`, scenario.reportProvider, scenario.reportStatus); err != nil {
					return err
				}
				entitlementStatus := scenario.entitlementStatus
				if entitlementStatus == "" {
					entitlementStatus = "active"
				}
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent-owned','intent-owned','customer-owned',?)`, entitlementStatus); err != nil {
					return err
				}
				sink := &entitlementDependencySink{ready: !scenario.blocked}
				worker, err := NewEntitlementCreatedWorker(database, entitlementDependencyRouter{sink: sink})
				if err != nil {
					return err
				}
				batch, err := worker.Run(ctx, 1)
				if err != nil {
					return err
				}
				if scenario.blocked {
					if batch.Selected != 0 || sink.calls != 0 {
						t.Fatalf("local dependency consumed a binding attempt: batch=%+v calls=%d", batch, sink.calls)
					}
					row, err := database.GetOne(ctx, `SELECT status_code,attempt_count FROM plugin_idc_core_entitlement_created_outbox`)
					if err != nil {
						return err
					}
					if row["status_code"].String() != "pending" || row["attempt_count"].Int() != 0 {
						t.Fatalf("pending dependency changed the delivery history: %v", row)
					}
					if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_resource_outcome_outbox SET status_code='reported' WHERE provider_task_no='resource-current'`); err != nil {
						return err
					}
					if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active'; UPDATE plugin_idc_core_product_fulfillment_task SET status_code='succeeded'`); err != nil {
						return err
					}
					sink.ready = true
					batch, err = worker.Run(ctx, 1)
					if err != nil {
						return err
					}
				}
				if batch.Delivered != 1 || sink.calls != 1 {
					t.Fatalf("ready or unrelated event did not retain normal delivery: batch=%+v calls=%d", batch, sink.calls)
				}
				row, err := database.GetOne(ctx, `SELECT status_code,attempt_count FROM plugin_idc_core_entitlement_created_outbox`)
				if err != nil {
					return err
				}
				if row["status_code"].String() != "delivered" || row["attempt_count"].Int() != 1 {
					t.Fatalf("binding did not deliver exactly once: %v", row)
				}
				batch, err = worker.Run(ctx, 1)
				if err != nil {
					return err
				}
				if batch.Selected != 0 || sink.calls != 1 {
					t.Fatal("delivered binding replayed")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
