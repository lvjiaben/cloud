// Failure receipts distinguish a proven no-object result from unknown provider
// outcomes and retain already-paid purchase evidence without automatic refunds.

package serveroperation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

func TestNativeLifecycleFailedCreationPostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "unknown", "native-a", "free", "manual_review")
	input := servercommercialcap.Completion{OperationNo: "unknown", EntitlementNo: "native-a", TaskNo: "task", Status: servercommercialcap.CompletionUnknown, EvidenceDigest: strings.Repeat("b", 64), ObservedAt: time.Now().UTC()}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteFailedAccessory(ctx, database, input) }); err == nil {
		t.Fatal("unknown result released the reservation")
	}
	if _, _, err := nativeAccessoryOccupancy(ctx, database, servercommercialcap.TargetSnapshot{EntitlementNo: "native-a", CustomerNo: "customer"}, "snapshot"); err == nil {
		t.Fatal("ambiguous creation permitted a new quote")
	}
	for _, mode := range []string{"free", "paid"} {
		nativeLifecycleIntent(t, database, "failed-"+mode, "native-b", mode, "failed")
		input.OperationNo, input.EntitlementNo, input.Status = "failed-"+mode, "native-b", servercommercialcap.CompletionFailed
		for range 2 {
			if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteFailedAccessory(ctx, database, input) }); err != nil {
				t.Fatal(err)
			}
		}
	}
	count, reserved, err := nativeAccessoryOccupancy(ctx, database, servercommercialcap.TargetSnapshot{EntitlementNo: "native-b", CustomerNo: "customer"}, "snapshot")
	if err != nil || count != 0 || reserved != 0 {
		t.Fatal("proven failed creation retained an occupied slot", err)
	}
	counted, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement
WHERE subject_no='native-b' AND status_code='creation_failed' AND object_no IS NULL AND deleted_at IS NULL
AND NOT auto_renew_enabled AND current_period_start_at IS NULL AND current_period_end_at IS NULL`)
	if err != nil || counted.Int() != 2 {
		t.Fatal("creation failure pretended to delete a real object", err)
	}
	counted, err = database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement_event WHERE event_type_code='creation_failed'`)
	if err != nil || counted.Int() != 2 {
		t.Fatal("failure replay duplicated terminal evidence", err)
	}
	counted, err = database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_no='failed-paid' AND financially_settled_at IS NOT NULL AND settlement_no='settled'`)
	if err != nil || counted.Int() != 1 {
		t.Fatal("failed paid create lost its financial trace", err)
	}
}
