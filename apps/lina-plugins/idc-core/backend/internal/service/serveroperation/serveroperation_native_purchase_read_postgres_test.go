// Checkout replay reads the original payable result without consulting live
// provider state. Current identity, ownership and the original key still apply.
package serveroperation

import (
	"context"
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestNativeCommercialReadPurchasePostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	input := servercommercialcap.PurchaseReadRequest{QuoteNo: p.QuoteNo, EntitlementNo: f.target.EntitlementNo, CustomerNo: f.target.CustomerNo, IdempotencyKey: "execution-native-checkout"}
	assert := func(status string) {
		t.Helper()
		got, err := f.native.ReadPurchase(f.ctx, input)
		if err != nil || got == nil || got.OperationNo != p.OperationNo || got.Status != status || !got.Reused {
			t.Fatalf("lost response replay: %+v %+v", got, err)
		}
	}
	assert("ready")
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "task_replay", nil }); err != nil {
		t.Fatal(err)
	}
	assert("executing")
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended',version=version+1 WHERE entitlement_no=?`, f.target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	assert("executing")
	for _, mutate := range []func(*servercommercialcap.PurchaseReadRequest){
		func(in *servercommercialcap.PurchaseReadRequest) { in.CustomerNo = "other-customer" },
		func(in *servercommercialcap.PurchaseReadRequest) { in.EntitlementNo = "other-entitlement" },
		func(in *servercommercialcap.PurchaseReadRequest) { in.IdempotencyKey = "different-original-key" },
	} {
		bad := input
		mutate(&bad)
		if _, err := f.native.ReadPurchase(f.ctx, bad); err == nil {
			t.Fatal("foreign identity/key recovered purchase")
		}
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 1)
}

func TestNativeCommercialReadPurchaseAbsentPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionSnapshotCreate, Parameters: json.RawMessage(`{"name":"not-yet-purchased"}`), IdempotencyKey: "native-unpurchased-quote"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.native.ReadPurchase(f.ctx, servercommercialcap.PurchaseReadRequest{QuoteNo: q.QuoteNo, EntitlementNo: f.target.EntitlementNo, CustomerNo: f.target.CustomerNo, IdempotencyKey: "native-unpurchased-key"})
	if err != nil || got != nil {
		t.Fatalf("unsubmitted purchase must be absent: %+v %+v", got, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 0)
}
