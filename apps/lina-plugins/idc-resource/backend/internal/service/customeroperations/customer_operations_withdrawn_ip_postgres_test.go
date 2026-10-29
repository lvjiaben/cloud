// This file proves cached, formerly conditional IP-count support cannot authorize
// an address replacement at display, quote validation, enqueue or dispatch.
package customeroperations

import (
	"context"
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/nativecommercial"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestWithdrawnZJMFReplacementRejectsStaleSnapshotAndQueuedWritePostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	if _, err := f.db.Exec(f.ctx, `CREATE TABLE test_native_commercial_intent(number text PRIMARY KEY,status text NOT NULL,task_no text);
 INSERT INTO test_native_commercial_intent VALUES('commercial-ip-old','ready',NULL);
 UPDATE plugin_idc_resource_capability_snapshot_item SET support_code='conditional',constraints='[{"name":"ip_num","required":true,"minimum":1}]' WHERE capability_code='server.ip.change'`); err != nil {
		t.Fatal(err)
	}
	route := serveroperationresourcecap.CapabilityRoute{ServiceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", PoolNo: "pool-main"}
	params := json.RawMessage(`{"ip_num":2}`)
	if err := f.source.ValidateParameters(f.ctx, route, serveroperationresourcecap.ActionChangeIP, params); err != nil {
		t.Fatal("unrelated provider incorrectly blocked", err)
	}
	owner := &commercialFixture{db: f.db, allowed: true, operation: servercommercialcap.Operation{OperationNo: "commercial-ip-old", EntitlementNo: "ent_" + strings.Repeat("1", 32), CustomerNo: "customer-a", LocalResourceNo: route.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo, ExpectedEntitlementVersion: 1, Action: serveroperationresourcecap.ActionChangeIP, Parameters: params, ContentDigest: strings.Repeat("b", 64)}}
	worker, err := nativecommercial.New(f.db, owner, f.source)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.RunReady(f.ctx); err != nil {
		t.Fatal(err)
	}
	store, err := resourcetask.NewStore(f.db)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "withdrawal-worker", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(tasks) != 1 {
		t.Fatal("old task missing", err)
	}
	task := tasks[0]
	// The persisted capability evidence remains conditional. Only the connector
	// identity changes to the exact withdrawn contract; other providers stay valid.
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_channel SET provider_plugin_id='idc-zjmfcloud' WHERE channel_no='channel-main'`); err != nil {
		t.Fatal(err)
	}
	advertised, err := f.source.ListCapabilities(f.ctx, []serveroperationresourcecap.CapabilityRoute{route})
	if err != nil || len(advertised) != 1 || slices.Contains(advertised[0].Actions, serveroperationresourcecap.ActionChangeIP) || !slices.Contains(advertised[0].Actions, serveroperationresourcecap.ActionStart) {
		t.Fatal("stale snapshot exposed withdrawn action or removed unrelated action", err)
	}
	if err = f.source.ValidateParameters(f.ctx, route, serveroperationresourcecap.ActionChangeIP, params); err == nil {
		t.Fatal("quote validation admitted withdrawn replacement")
	}
	if _, err = f.source.CreateTask(f.ctx, serveroperationresourcecap.TaskRequest{OperationNo: "new-ip-blocked", ServiceNo: route.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo, Action: serveroperationresourcecap.ActionChangeIP, Parameters: params, ExpectedServiceVersion: 1, RequestDigest: strings.Repeat("c", 64)}); err == nil {
		t.Fatal("new task admitted withdrawn replacement")
	}
	gate := worker.Admission(func(context.Context, resourcetask.ClaimedTask, resourcecap.OperationRequest, resourcecap.Provider, func(context.Context) error) error {
		t.Fatal("fell through to legacy execution")
		return nil
	})
	writes := 0
	err = gate(f.ctx, task, resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityChangeIP, ResourceRef: "external-1", Parameters: params}, f.provider, func(context.Context) error { writes++; return nil })
	if err == nil || writes != 0 {
		t.Fatal("queued old contract reached supplier")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='result_unknown',result_unknown=true,updated_at=CURRENT_TIMESTAMP WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	if err = worker.RunOutcomes(f.ctx); err != nil {
		t.Fatal(err)
	}
	if len(owner.receipts) != 1 || owner.receipts[0].Status != servercommercialcap.CompletionUnknown {
		t.Fatal("historical unknown became release/refund evidence", owner.receipts)
	}
	count, err := f.db.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_resource_task WHERE capability_code='server.ip.change'`)
	if err != nil || count.Int() != 1 {
		t.Fatal("replacement was re-enqueued", err)
	}
}
