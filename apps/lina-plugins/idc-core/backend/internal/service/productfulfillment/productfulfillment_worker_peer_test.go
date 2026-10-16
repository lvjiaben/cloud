// These rollback-only PostgreSQL tests prove multi-service results create exact
// immutable local mappings, while one-time delivery never invents a service.

package productfulfillment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

func TestPeerDeliveryPersistencePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(context.Background()); closeErr != nil {
			t.Error(closeErr)
		}
	})
	rollback := errors.New("rollback")
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		_, setupErr := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(tenant_id INT,task_no TEXT,upstream_deliveries JSONB DEFAULT '[]'::jsonb);
 CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT,intent_no TEXT,customer_no TEXT,order_no TEXT,order_item_no TEXT,product_no TEXT,version_no TEXT,unit_sequence INT,configuration_snapshot_no TEXT,interface_instance_no TEXT,plugin_id TEXT,plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,ongoing_management BOOLEAN,renewal_mode_code TEXT);
 CREATE TEMP TABLE plugin_idc_core_product_delivery_record(tenant_id INT,delivery_no TEXT,order_no TEXT,order_item_no TEXT,customer_no TEXT);
 CREATE TEMP TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT PRIMARY KEY,delivery_no TEXT,source_intent_no TEXT,customer_no TEXT,order_no TEXT,order_item_no TEXT,product_no TEXT,version_no TEXT,unit_sequence INT,configuration_snapshot_no TEXT,interface_instance_no TEXT,plugin_id TEXT,plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,status_code TEXT,started_at TIMESTAMPTZ,expires_at TIMESTAMPTZ,upstream_order_reference TEXT,upstream_delivery_no TEXT,upstream_service_no TEXT,UNIQUE(source_intent_no,upstream_service_no));
 INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'task','[]');
 INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'intent','customer','order','item','product','version',1,'config','interface','idc-peer','v0.1.0','v1',repeat('a',64),TRUE,'provider_confirmed');
 INSERT INTO plugin_idc_core_product_delivery_record VALUES(0,'delivery','order','item','customer');`)
		if setupErr != nil {
			t.Fatal(setupErr)
		}
		// Install the unmodified production 078 task update guard on this
		// transaction-local table, including its real version/state/lease fields.
		if _, setupErr = db.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_task
 ADD COLUMN intent_no TEXT DEFAULT 'intent', ADD COLUMN status_code TEXT DEFAULT 'querying',
 ADD COLUMN phase_code TEXT DEFAULT 'post_submit_query', ADD COLUMN attempt_count INT DEFAULT 1,
 ADD COLUMN query_attempt_count INT DEFAULT 1, ADD COLUMN submit_attempt_count INT DEFAULT 0,
 ADD COLUMN maximum_attempts INT DEFAULT 10, ADD COLUMN retry_base_seconds INT DEFAULT 30,
 ADD COLUMN next_attempt_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
 ADD COLUMN lease_token TEXT DEFAULT 'lease', ADD COLUMN lease_expires_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP+INTERVAL '1 minute',
 ADD COLUMN external_reference TEXT DEFAULT '', ADD COLUMN last_result_code TEXT DEFAULT '',
 ADD COLUMN takeover_requested BOOL DEFAULT FALSE, ADD COLUMN cancel_requested BOOL DEFAULT FALSE,
 ADD COLUMN version BIGINT DEFAULT 2, ADD COLUMN created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
 ADD COLUMN updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;`); setupErr != nil {
			t.Fatal(setupErr)
		}
		_, file, _, _ := runtime.Caller(0)
		migration, readErr := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/078-product-fulfillment-execution.sql"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(migration)
		start := strings.Index(text, "CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_task()")
		end := strings.Index(text[start:], "$$ LANGUAGE plpgsql;") + start + len("$$ LANGUAGE plpgsql;")
		guard := strings.Replace(text[start:end], "plugin_idc_core_guard_fulfillment_task()", "pg_temp.peer_task_guard()", 1)
		if _, setupErr = db.Exec(ctx, guard+`CREATE TRIGGER peer_task_guard BEFORE UPDATE ON plugin_idc_core_product_fulfillment_task FOR EACH ROW EXECUTE FUNCTION pg_temp.peer_task_guard();`); setupErr != nil {
			t.Fatal(setupErr)
		}
		worker := &fulfillmentWorker{database: db}
		claim := &claimedFulfillment{PluginID: renewalownercap.PeerOwnerID, IntentNo: "intent", TaskNo: "task", LeaseToken: "lease", Action: productfulfillmentcap.ActionQuery}
		result := &productfulfillmentcap.Result{Version: "v1", Status: productfulfillmentcap.StatusSucceeded, ExternalReference: "peer-order:42", Deliveries: []productfulfillmentcap.Delivery{
			{UpstreamDeliveryNo: "delivery-first", UpstreamServiceNo: "service-first", StartedAt: 1700000000000, ExpiresAt: 1800000000000},
			{UpstreamDeliveryNo: "delivery-second", UpstreamServiceNo: "service-second", StartedAt: 1700000001000, ExpiresAt: 1800000001000},
		}}
		guardErr := db.Transaction(ctx, func(saveCtx context.Context, _ gdb.TX) error {
			_, updateErr := db.Exec(saveCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET upstream_deliveries='[{"upstreamServiceNo":"unauthorized"}]',version=version+1 WHERE task_no='task'`)
			return updateErr
		})
		if guardErr == nil || !strings.Contains(guardErr.Error(), "must advance its state") {
			t.Fatalf("production 078 state guard not exercised: %v", guardErr)
		}
		if worker.finalizeTaskState(ctx, claim, 1, workerDecision{TaskStatus: "succeeded", PhaseCode: "post_submit_query"}, result.ExternalReference, "succeeded", result) == nil {
			t.Fatal("stale version froze deliveries")
		}
		claim.LeaseToken = "foreign"
		if worker.finalizeTaskState(ctx, claim, 2, workerDecision{TaskStatus: "succeeded", PhaseCode: "post_submit_query"}, result.ExternalReference, "succeeded", result) == nil {
			t.Fatal("foreign lease froze deliveries")
		}
		claim.LeaseToken = "lease"
		for index := range 2 {
			if storeErr := worker.storePeerDeliveries(ctx, claim, result); storeErr != nil {
				t.Fatal(storeErr)
			}
			if index == 0 {
				if stateErr := worker.finalizeTaskState(ctx, claim, 2, workerDecision{TaskStatus: "succeeded", PhaseCode: "post_submit_query"}, result.ExternalReference, "succeeded", result); stateErr != nil {
					t.Fatal(stateErr)
				}
			}
		}
		frozen, queryErr := db.GetValue(ctx, `SELECT status_code||':'||version||':'||jsonb_array_length(upstream_deliveries) FROM plugin_idc_core_product_fulfillment_task WHERE task_no='task'`)
		if queryErr != nil || frozen.String() != "succeeded:3:2" {
			t.Fatalf("snapshot not frozen atomically with terminal state: %v %v", frozen, queryErr)
		}
		if worker.finalizeTaskState(ctx, claim, 2, workerDecision{TaskStatus: "succeeded", PhaseCode: "post_submit_query"}, result.ExternalReference, "succeeded", result) == nil {
			t.Fatal("terminal state replay advanced task twice")
		}
		count, queryErr := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE upstream_order_reference='peer-order:42'`)
		if queryErr != nil || count.Int() != 2 {
			t.Fatalf("multi-service or replay: count=%v err=%v", count, queryErr)
		}
		exact, queryErr := db.GetValue(ctx, `SELECT upstream_delivery_no||':'||extract(epoch FROM expires_at)*1000 FROM plugin_idc_core_service_entitlement WHERE upstream_service_no='service-second'`)
		if queryErr != nil || exact.String() != "delivery-second:1800000001000.000000" {
			t.Fatalf("exact second mapping: %v %v", exact, queryErr)
		}
		number, queryErr := db.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE upstream_service_no='service-second'`)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		event := entitlementcreatedcap.Event{Version: entitlementcreatedcap.ContractVersion, EventNo: "event", EntitlementNo: number.String(), IntentNo: "intent", CustomerNo: "customer", ProductNo: "product", ProductVersionNo: "version", ProductNameZhCN: "产品", ProductNameEnUS: "Product", ConfigurationSnapshotNo: "config", InterfaceInstanceNo: "interface", PluginID: "idc-peer", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", DeclarationDigest: strings.Repeat("a", 64), OwnerID: renewalownercap.PeerOwnerID}
		sink, sinkErr := NewPeerEntitlementCreatedSink(db)
		if sinkErr != nil {
			t.Fatal(sinkErr)
		}
		receipt, sinkErr := sink.Deliver(ctx, event)
		if sinkErr != nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) || receipt.OwnerReference != "service-second" {
			t.Fatalf("local peer acknowledgement: %+v %v", receipt, sinkErr)
		}
		repeated, sinkErr := sink.Deliver(ctx, event)
		if sinkErr != nil || repeated.EvidenceDigest != receipt.EvidenceDigest {
			t.Fatalf("receipt replay: %+v %v", repeated, sinkErr)
		}
		event.CustomerNo = "other-customer"
		if _, sinkErr = sink.Deliver(ctx, event); sinkErr == nil {
			t.Fatal("wrong customer event accepted")
		}
		result.Deliveries[1].UpstreamServiceNo = "service-changed"
		if worker.storePeerDeliveries(ctx, claim, result) == nil {
			t.Fatal("changed mapping accepted")
		}
		result.Deliveries[1].UpstreamServiceNo = "service-first"
		if worker.storePeerDeliveries(ctx, claim, result) == nil {
			t.Fatal("duplicate service accepted")
		}
		if _, setupErr = db.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET ongoing_management=FALSE,renewal_mode_code='unavailable'; INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,upstream_deliveries) VALUES(0,'once','[]')`); setupErr != nil {
			t.Fatal(setupErr)
		}
		claim.TaskNo = "once"
		result.Deliveries = []productfulfillmentcap.Delivery{{UpstreamDeliveryNo: "one-time", StartedAt: 1700000000000}}
		if storeErr := worker.storePeerDeliveries(ctx, claim, result); storeErr != nil {
			t.Fatal(storeErr)
		}
		if stateErr := worker.finalizeTaskState(ctx, claim, 2, workerDecision{TaskStatus: "succeeded", PhaseCode: "post_submit_query"}, result.ExternalReference, "succeeded", result); stateErr != nil {
			t.Fatal(stateErr)
		}
		count, queryErr = db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_service_entitlement`)
		if queryErr != nil || count.Int() != 2 {
			t.Fatal("one-time delivery invented an entitlement")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
