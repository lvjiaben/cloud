// Admission tests use actual native quotes and owner aggregates to prove that
// confirmation and rejection cannot change the frozen target or fake execution.
package serveroperation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestNativeCommercialReadCheckoutPostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	input := servercommercialcap.CheckoutRequest{QuoteNo: p.QuoteNo, Target: f.target}
	got, err := f.native.ReadCheckout(f.ctx, input)
	if err != nil || got == nil || got.Action != serveroperationresourcecap.ActionSnapshotCreate || got.EntitlementNo != f.target.EntitlementNo || got.LocalResourceNo != f.target.LocalResourceNo || got.TotalAmountMinor != 0 {
		t.Fatalf("read frozen confirmation: %+v %+v", got, err)
	}
	for _, mutate := range []func(*servercommercialcap.CheckoutRequest){
		func(in *servercommercialcap.CheckoutRequest) { in.Target.CustomerNo = "other-customer" },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.EntitlementNo = "other-entitlement" },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.LocalResourceNo = "other-local-resource" },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.ChannelNo = "other-channel" },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.PoolNo = "other-pool" },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.EntitlementVersion++ },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.Actions = nil },
		func(in *servercommercialcap.CheckoutRequest) { in.Target.ExpiresAt = time.Now().Add(-time.Minute) },
	} {
		bad := input
		mutate(&bad)
		if _, err = f.native.ReadCheckout(f.ctx, bad); err == nil {
			t.Fatal("mismatched confirmation accepted")
		}
	}
	f.assertCount(t, `SELECT count(*) FROM test_native_task`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event`, 1)
}

func TestNativeCommercialRespectsOrdinaryWritePostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	write := servercommercialcap.QueuedWrite{OperationNo: "nso_commercial_fence", TaskNo: "task_commercial_fence", EntitlementNo: f.target.EntitlementNo, CustomerNo: f.target.CustomerNo, ExpectedEntitlementVersion: f.target.EntitlementVersion, Action: serveroperationresourcecap.ActionReboot, RequestDigest: strings.Repeat("a", 64)}
	if err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error { return f.native.HoldQueuedWrite(ctx, write) }); err != nil {
		t.Fatal(err)
	}
	quote := servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionSnapshotCreate, Parameters: []byte(`{"name":"after-free-operation"}`), IdempotencyKey: "native-ordinary-write-mutex"}
	if _, err := f.native.Quote(f.ctx, quote); err == nil {
		t.Fatal("commercial quote crossed active ordinary write")
	}
	completion := servercommercialcap.QueuedWriteCompletion{TaskNo: write.TaskNo, EntitlementNo: write.EntitlementNo, CustomerNo: write.CustomerNo, Status: servercommercialcap.CompletionUnknown, EvidenceDigest: strings.Repeat("b", 64)}
	if err := f.native.CompleteQueuedWrite(f.ctx, completion); err != nil {
		t.Fatal(err)
	}
	if _, err := f.native.Quote(f.ctx, quote); err == nil {
		t.Fatal("commercial quote crossed unknown ordinary write")
	}
	completion.Status = servercommercialcap.CompletionSucceeded
	completion.EvidenceDigest = strings.Repeat("c", 64)
	if err := f.native.CompleteQueuedWrite(f.ctx, completion); err != nil {
		t.Fatal(err)
	}
	if _, err := f.native.Quote(f.ctx, quote); err != nil {
		t.Fatalf("verified completion did not release commercial fence: %+v", err)
	}
}

func TestNativeCommercialRejectUnsentPostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	input := servercommercialcap.AdmissionFailure{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, ReasonCode: "capability_unavailable", EvidenceDigest: strings.Repeat("a", 64)}
	if err := f.native.RejectUnsent(f.ctx, input); err != nil {
		t.Fatal(err)
	}
	if err := f.native.RejectUnsent(f.ctx, input); err != nil {
		t.Fatalf("identical replay: %+v", err)
	}
	bad := input
	bad.EvidenceDigest = strings.Repeat("b", 64)
	if err := f.native.RejectUnsent(f.ctx, bad); err == nil {
		t.Fatal("changed evidence accepted")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review' AND provider_task_no IS NULL AND execution_started_at IS NULL AND completed_at IS NULL`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event WHERE to_status_code='manual_review'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='pending_binding'`, 1)
	ready, err := f.native.ListReady(f.ctx, 8)
	if err != nil || len(ready) != 0 {
		t.Fatalf("rejected operation dispatched: %v %+v", ready, err)
	}
}

func TestNativeCommercialRejectStartedPostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "bound-task", nil }); err != nil {
		t.Fatal(err)
	}
	if err := f.native.RejectUnsent(f.ctx, servercommercialcap.AdmissionFailure{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, ReasonCode: "resource_unavailable", EvidenceDigest: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("bound task marked undispatched")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='executing' AND provider_task_no='bound-task'`, 1)
}

func TestNativeCommercialReadyRotationPostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_business_operation SET updated_at=CURRENT_TIMESTAMP-INTERVAL '1 day' WHERE operation_no=?`, p.OperationNo); err != nil {
		t.Fatal(err)
	}
	ready, err := f.native.ListReady(f.ctx, 1)
	if err != nil || len(ready) != 1 || ready[0] != p.OperationNo {
		t.Fatalf("ready batch: %v %+v", ready, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='ready' AND updated_at>CURRENT_TIMESTAMP-INTERVAL '1 minute' AND provider_task_no IS NULL`, 1)
	if err = f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, e := f.native.ListReady(ctx, 1); e == nil {
			t.Error("dispatcher allowed caller-owned transaction")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
