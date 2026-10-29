// Restore must not treat a newly observed unbound paid object as grandfathered
// upstream capacity. Real accessory constraints and persisted operation fences
// distinguish unstarted purchases, ambiguous creates, and proven old bindings.

package serveroperation

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestNativeLifecycleUnboundRestoreFencePostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "known-free", "native-a", "free", "succeeded")
	known := servercommercialcap.Completion{OperationNo: "known-free", EntitlementNo: "native-a", ObjectNo: "old-free-object", Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, known) }); err != nil {
		t.Fatal(err)
	}
	nativeLifecycleIntent(t, database, "uncertain-create", "native-a", "paid", "awaiting_financial_settlement")
	object := servercommercialcap.ObjectRequest{EntitlementNo: "native-a", CustomerNo: "customer", Kind: serveroperationresourcecap.AccessoryKindSnapshot, ObjectNo: "new-unbound-object"}
	for _, state := range []string{"awaiting_financial_settlement", "ready", "manual_review"} {
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code=? WHERE operation_no='uncertain-create'`, state); err != nil {
			t.Fatal(err)
		}
		if err := nativeCheckRestore(ctx, database, object); err != nil {
			t.Fatalf("unsent %s incorrectly blocked existing upstream object: %v", state, err)
		}
	}
	for _, state := range []string{"executing", "manual_review", "succeeded"} {
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code=?,provider_task_no='actual-create-task',execution_started_at=CURRENT_TIMESTAMP WHERE operation_no='uncertain-create'`, state); err != nil {
			t.Fatal(err)
		}
		if err := nativeCheckRestore(ctx, database, object); err == nil {
			t.Fatalf("unbound %s creation bypassed restore charge/unknown fence", state)
		}
		old := object
		old.ObjectNo = known.ObjectNo
		if err := nativeCheckRestore(ctx, database, old); err != nil {
			t.Fatalf("proven old free object blocked: %v", err)
		}
		otherKind := object
		otherKind.Kind = serveroperationresourcecap.AccessoryKindBackup
		if err := nativeCheckRestore(ctx, database, otherKind); err != nil {
			t.Fatalf("other object kind blocked: %v", err)
		}
		otherServer := object
		otherServer.EntitlementNo = "native-b"
		if err := nativeCheckRestore(ctx, database, otherServer); err != nil {
			t.Fatalf("other server blocked: %v", err)
		}
	}
	// Listing/deleting an untracked number cannot bind it or erase the unknown
	// create's fence. A missing billing object is only an idempotent delete.
	deletion := known
	deletion.OperationNo = "delete-unbound"
	deletion.ObjectNo = object.ObjectNo
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteDeletedAccessory(ctx, database, object, deletion)
	}); err != nil {
		t.Fatal(err)
	}
	if err := nativeCheckRestore(ctx, database, object); err == nil {
		t.Fatal("untracked deletion cleared an unrelated unbound create")
	}
	// A real old binding may be deleted concurrently, but neither its slot
	// release nor a refreshed listing can identify the new unknown object.
	var wait sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		old := object
		old.ObjectNo = known.ObjectNo
		receipt := known
		receipt.OperationNo = "delete-old-free"
		errs <- database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			return nativeCompleteDeletedAccessory(ctx, database, old, receipt)
		})
	}()
	go func() {
		defer wait.Done()
		<-start
		for range 20 {
			if nativeCheckRestore(ctx, database, object) == nil {
				errs <- ErrOperationConflict
				return
			}
		}
		errs <- nil
	}()
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent deletion bypassed unbound restore fence: %v", err)
		}
	}
	if err := nativeCheckRestore(ctx, database, object); err == nil {
		t.Fatal("committed old deletion cleared new unknown fence")
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='failed' WHERE operation_no='uncertain-create'`); err != nil {
		t.Fatal(err)
	}
	failed := known
	failed.OperationNo = "uncertain-create"
	failed.Status = servercommercialcap.CompletionFailed
	failed.ObjectNo = ""
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteFailedAccessory(ctx, database, failed) }); err != nil {
		t.Fatal(err)
	}
	if err := nativeCheckRestore(ctx, database, object); err != nil {
		t.Fatalf("proven no-object failure retained ambiguous fence: %v", err)
	}
}

func TestNativeLifecycleParentRestoreFencePostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "parent-restore", "native-a", "free", "succeeded")
	known := servercommercialcap.Completion{OperationNo: "parent-restore", EntitlementNo: "native-a", ObjectNo: "old-free-object", Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("b", 64), ObservedAt: time.Now().UTC()}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, known) }); err != nil {
		t.Fatal(err)
	}
	bound := servercommercialcap.ObjectRequest{EntitlementNo: "native-a", CustomerNo: "customer", Kind: serveroperationresourcecap.AccessoryKindSnapshot, ObjectNo: known.ObjectNo}
	untracked := bound
	untracked.ObjectNo = "preexisting-untracked"
	assertRestore := func(allowed bool) {
		t.Helper()
		for _, object := range []servercommercialcap.ObjectRequest{bound, untracked} {
			if err := nativeCheckRestore(ctx, database, object); (err == nil) != allowed {
				t.Fatalf("restore=%t object=%s error=%v", allowed, object.ObjectNo, err)
			}
		}
	}
	assertRestore(true)
	for _, state := range []string{"pending_activation", "suspended", "expired", "terminated", "cancelled"} {
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code=? WHERE entitlement_no='native-a'`, state); err != nil {
			t.Fatal(err)
		}
		assertRestore(false)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE entitlement_no='native-a'`); err != nil {
		t.Fatal(err)
	}
	assertRestore(false)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET expires_at=CURRENT_TIMESTAMP+INTERVAL '1 hour' WHERE entitlement_no='native-a';
INSERT INTO plugin_idc_core_refund_entitlement_lock(tenant_id,entitlement_no) VALUES(0,'native-a')`); err != nil {
		t.Fatal(err)
	}
	assertRestore(false)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_refund_entitlement_lock SET released_at=CURRENT_TIMESTAMP WHERE entitlement_no='native-a'`); err != nil {
		t.Fatal(err)
	}
	assertRestore(true)
}
