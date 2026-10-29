// Native settlement tests challenge real SQL transactions without manufacturing legacy service records or provider calls.

package serveroperation

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

func nativeAccessorySettlementFixture(t *testing.T, kind string) (gdb.DB, fulfillment.SettlementEvent, *billing.FinancialSettlement) {
	t.Helper()
	database, input, proof := accessorySettlementFixture(t, kind)
	action, parameters, runtimeStatus := kind+"_create", `{"name": "test"}`, "stopped"
	if kind == "traffic" {
		action, parameters, runtimeStatus = "traffic_add", `{"trafficGb": 100}`, "suspended"
	}
	now := time.Now().UTC()
	target := servercommercialcap.TargetSnapshot{EntitlementNo: "ENT_native", CustomerNo: "cus_1",
		LocalResourceNo: "RESOURCE_native", ChannelNo: "channel_1", PoolNo: "pool_1", EntitlementVersion: 2,
		RuntimeStatus: runtimeStatus, TrafficSuspended: kind == "traffic", SnapshotNo: "snapshot_native",
		ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour), Actions: []serveroperationresourcecap.Action{serveroperationresourcecap.Action(action)}}
	frozen, err := json.Marshal(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), `DELETE FROM plugin_idc_core_service_instance;
        INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,currency_code,version,status_code,entitlement_owner_id,upstream_service_no)
        VALUES ('ENT_native','cus_1','CNY',2,'active','idc-resource','');
        UPDATE plugin_idc_core_business_operation SET service_no=NULL,subject_type_code='entitlement',subject_no='ENT_native',expected_subject_version=2,expected_service_version=NULL,channel_no=NULL;
        UPDATE plugin_idc_core_service_price_snapshot SET service_no=NULL,subject_type_code='entitlement',subject_no='ENT_native';`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_core_server_accessory_quote SET service_no=NULL,
        subject_type_code='entitlement',subject_no='ENT_native',expected_subject_version=2,native_target_snapshot=?::JSONB`, string(frozen)); err != nil {
		t.Fatal(err)
	}
	row := &accessorySettlementRow{SubjectTypeCode: nativeAccessorySubject, SubjectNo: "ENT_native",
		OperationNo: "aop_1", QuoteNo: "quo_1", OrderNo: "ord_1", InvoiceNo: "inv_1", ActionCode: action,
		Parameters: parameters, ExpectedServiceVersion: 2}
	content, err := accessorySettlementIntentDigest(row)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_core_business_operation SET content_digest=?`, content); err != nil {
		t.Fatal(err)
	}
	return database, input, proof
}

func TestNativeAccessorySettlementConcurrentReplayPostgreSQL(t *testing.T) {
	for _, kind := range []string{"traffic", "snapshot", "backup"} {
		t.Run(kind, func(t *testing.T) {
			database, input, proof := nativeAccessorySettlementFixture(t, kind)
			consumer, err := NewAccessorySettlementConsumer(database, accessorySettlementEvidenceStub{proof: proof})
			if err != nil {
				t.Fatal(err)
			}
			var wait sync.WaitGroup
			failures := make(chan error, 4)
			fresh := make(chan bool, 4)
			for range 4 {
				wait.Add(1)
				go func() {
					defer wait.Done()
					result, handled, consumeErr := consumer.ConsumeAddonSettlement(context.Background(), input)
					if consumeErr != nil {
						failures <- consumeErr
						return
					}
					if !handled || result == nil || result.FulfillmentCount != 0 {
						t.Error("native addon was not consumed as zero new services")
						return
					}
					fresh <- !result.Reused
				}()
			}
			wait.Wait()
			close(failures)
			close(fresh)
			for consumeErr := range failures {
				t.Error(consumeErr)
			}
			newCount := 0
			for first := range fresh {
				if first {
					newCount++
				}
			}
			if newCount != 1 {
				t.Fatalf("fresh native deliveries=%d want=1", newCount)
			}
			assertNativeAccessoryCounts(t, database, 1, 1)
			if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_core_business_operation SET status_code='succeeded',version=version+1;
                UPDATE plugin_idc_core_service_entitlement SET version=3`); err != nil {
				t.Fatal(err)
			}
			result, handled, err := consumer.ConsumeAddonSettlement(t.Context(), input)
			if err != nil || !handled || result == nil || !result.Reused {
				t.Fatalf("terminal native replay=%+v err=%v", result, err)
			}
			state, err := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_core_business_operation`)
			if err != nil || state.String() != "succeeded" {
				t.Fatalf("replay reset resource outcome: state=%v err=%v", state, err)
			}
		})
	}
}

func TestNativeAccessorySettlementRejectsBrokenEvidencePostgreSQL(t *testing.T) {
	for _, check := range []struct{ name, sql string }{
		{"foreign owner", `UPDATE plugin_idc_core_service_entitlement SET customer_no='cus_other'`},
		{"foreign quote subject", `UPDATE plugin_idc_core_server_accessory_quote SET subject_no='ENT_other'`},
		{"legacy alias", `UPDATE plugin_idc_core_business_operation SET service_no='ENT_native'`},
		{"stale entitlement", `UPDATE plugin_idc_core_service_entitlement SET version=3`},
		{"wrong invoice total", `UPDATE plugin_idc_core_billing_invoice SET total_amount_minor=101`},
		{"missing settlement", `DELETE FROM plugin_idc_core_financial_settlement`},
		{"wrong payload", `UPDATE plugin_idc_core_outbox_event SET payload=jsonb_set(payload,'{totalAmountMinor}','101')`},
		{"changed parameters", `UPDATE plugin_idc_core_server_accessory_quote SET parameters='{"trafficGb":101}'::JSONB`},
		{"wrong resource identity", `UPDATE plugin_idc_core_server_accessory_quote SET native_target_snapshot=jsonb_set(native_target_snapshot,'{EntitlementNo}','"ENT_other"')`},
		{"unpaid invoice", `UPDATE plugin_idc_core_billing_invoice SET status_code='pending_payment'`},
	} {
		t.Run(check.name, func(t *testing.T) {
			database, input, proof := nativeAccessorySettlementFixture(t, "traffic")
			if _, err := database.Exec(t.Context(), check.sql); err != nil {
				t.Fatal(err)
			}
			consumer, err := NewAccessorySettlementConsumer(database, accessorySettlementEvidenceStub{proof: proof})
			if err != nil {
				t.Fatal(err)
			}
			if _, handled, err := consumer.ConsumeAddonSettlement(t.Context(), input); err == nil || !handled {
				t.Fatalf("native addon fell through or accepted corrupt evidence: handled=%v err=%v", handled, err)
			}
			assertNativeAccessoryCounts(t, database, 0, 0)
		})
	}
}

func TestNativeAccessorySettlementAuditRollbackPostgreSQL(t *testing.T) {
	database, input, proof := nativeAccessorySettlementFixture(t, "traffic")
	consumer, err := NewAccessorySettlementConsumer(database, accessorySettlementEvidenceStub{proof: proof})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), `ALTER TABLE plugin_idc_core_business_operation_event ADD CONSTRAINT reject_event CHECK(FALSE)`); err != nil {
		t.Fatal(err)
	}
	if _, handled, err := consumer.ConsumeAddonSettlement(t.Context(), input); err == nil || !handled {
		t.Fatal("native audit failure was hidden")
	}
	assertNativeAccessoryCounts(t, database, 0, 0)
	if _, err = database.Exec(t.Context(), `ALTER TABLE plugin_idc_core_business_operation_event DROP CONSTRAINT reject_event`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = consumer.ConsumeAddonSettlement(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	assertNativeAccessoryCounts(t, database, 1, 1)
}

func assertNativeAccessoryCounts(t *testing.T, database gdb.DB, events, receipts int) {
	t.Helper()
	for _, check := range []struct {
		table string
		count int
	}{
		{"plugin_idc_core_service_instance", 0}, {"plugin_idc_core_server_operation_execution", 0},
		{"plugin_idc_core_business_operation_event", events}, {"plugin_idc_core_financial_settlement_delivery", receipts},
	} {
		value, err := database.GetValue(t.Context(), `SELECT count(*) FROM `+check.table)
		if err != nil || value.Int() != check.count {
			t.Fatalf("native %s count=%v want=%d err=%v", check.table, value, check.count, err)
		}
	}
}
