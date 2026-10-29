// This file proves retained commercial reads do not depend on active provider
// channels, operability or fresh resource snapshots and never leak across owners.
package customeroperations

import (
	"context"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"testing"
	"time"
)

type commercialReadOwner struct {
	servercommercialcap.Service
	operations servercommercialcap.OperationListRequest
	billing    servercommercialcap.AccessoryBillingListRequest
	calls      int
}

func (o *commercialReadOwner) ListOperations(_ context.Context, in servercommercialcap.OperationListRequest) (*servercommercialcap.OperationPage, error) {
	o.calls++
	o.operations = in
	return &servercommercialcap.OperationPage{Items: []servercommercialcap.OperationHistory{}, NextCursor: "next-operations"}, nil
}
func (o *commercialReadOwner) ListAccessoryBilling(_ context.Context, in servercommercialcap.AccessoryBillingListRequest) (*servercommercialcap.AccessoryBillingPage, error) {
	o.calls++
	o.billing = in
	return &servercommercialcap.AccessoryBillingPage{Items: []servercommercialcap.AccessoryBilling{}, NextCursor: "next-billing"}, nil
}
func TestNativeCommercialReadSurvivesUnavailableRuntimePostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	f.customers.eligible = false
	f.provider.delay = time.Minute
	if _, err := f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_channel SET status_code='disabled';DELETE FROM plugin_idc_resource_capability_snapshot_item`); err != nil {
		t.Fatal(err)
	}
	owner := &commercialReadOwner{}
	service, err := NewCommercial(f.service, owner)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := service.CommercialOperations(f.ctx, CommercialHistoryQuery{ServerNo: f.serverNo, OperationNo: "owned-operation", Cursor: "opaque-operations", Limit: 7})
	if err != nil || operations.NextCursor != "next-operations" || owner.operations.CustomerNo != "customer-a" || owner.operations.EntitlementNo == "" || owner.operations.OperationNo != "owned-operation" || owner.operations.Cursor != "opaque-operations" || owner.operations.Limit != 7 {
		t.Fatal("history changed identity or filter", err)
	}
	billing, err := service.AccessoryBilling(f.ctx, AccessoryBillingQuery{ServerNo: f.serverNo, ObjectNo: "owned-object", Kind: "snapshot", Cursor: "opaque-billing", Limit: 9})
	if err != nil || billing.NextCursor != "next-billing" || owner.billing.CustomerNo != "customer-a" || owner.billing.EntitlementNo != owner.operations.EntitlementNo || owner.billing.ObjectNo != "owned-object" || string(owner.billing.Kind) != "snapshot" || owner.billing.Cursor != "opaque-billing" || owner.billing.Limit != 9 {
		t.Fatal("billing changed identity or filter", err)
	}
	before := owner.calls
	if _, err = service.CommercialOperations(f.ctx, CommercialHistoryQuery{ServerNo: f.serverNo, Limit: 101}); err == nil {
		t.Fatal("unbounded history accepted")
	}
	if _, err = service.AccessoryBilling(f.ctx, AccessoryBillingQuery{ServerNo: f.serverNo, Kind: "disk", Limit: 20}); err == nil {
		t.Fatal("unsupported billing kind accepted")
	}
	f.customers.customer = "customer-b"
	if _, err = service.CommercialOperations(f.ctx, CommercialHistoryQuery{ServerNo: f.serverNo, Limit: 20}); err == nil {
		t.Fatal("foreign commercial history exposed")
	}
	if _, err = service.AccessoryBilling(f.ctx, AccessoryBillingQuery{ServerNo: f.serverNo, Limit: 20}); err == nil {
		t.Fatal("foreign accessory billing exposed")
	}
	if owner.calls != before || f.provider.calls != 0 {
		t.Fatal("invalid/foreign read reached owner or any read called supplier")
	}
}
