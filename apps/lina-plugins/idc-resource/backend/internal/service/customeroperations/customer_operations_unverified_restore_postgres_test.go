// This file covers cached false readiness at the public directory, source
// validation, new task and already queued restore admission boundaries.
package customeroperations

import (
	"context"
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestUnverifiedZJMFRestoreRejectsCachedReadyAndQueuedWritesPostgreSQL(t *testing.T) {
	for _, action := range []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupRestore} {
		t.Run(string(action), func(t *testing.T) {
			f := newManagementFixture(t)
			f.provider.state = resourcecap.ResourceStateStopped
			kind, capability, deleteAction := resourcecap.AccessoryKindSnapshot, resourcecap.CapabilityManageSnapshot, serveroperationresourcecap.ActionSnapshotDelete
			if action == serveroperationresourcecap.ActionBackupRestore {
				kind, capability, deleteAction = resourcecap.AccessoryKindBackup, resourcecap.CapabilityBackup, serveroperationresourcecap.ActionBackupDelete
			}
			if _, err := f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_capability_snapshot_item SET support_code='conditional',constraints='[{"name":"action","required":true,"allowedValues":["create","delete","restore"]},{"name":"snapshot_ref","minimum":1}]' WHERE capability_code=?`, capability); err != nil {
				t.Fatal(err)
			}
			directory, err := accessorydirectory.New(f.db)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			published, err := directory.Publish(f.ctx, accessorydirectory.PublishInput{MappingNo: "mapping-1", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", ExternalResourceRef: "external-1", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(time.Minute), Items: []resourcecap.AccessoryItem{{Kind: kind, State: resourcecap.AccessoryStateAvailable, ExternalRef: "77", Data: json.RawMessage(`{"status":"ready"}`)}}}})
			if err != nil {
				t.Fatal(err)
			}
			params, err := json.Marshal(map[string]string{"objectNo": published.Items[0].ObjectNo, "confirmedServiceNo": f.serverNo})
			if err != nil {
				t.Fatal(err)
			}
			command := Command{ServerNo: f.serverNo, Action: string(action), Parameters: params, IdempotencyKey: "queued-before-readiness-withdrawal", Confirmed: true}
			if _, err = f.service.Create(f.ctx, command); err != nil {
				t.Fatal("unrelated provider restore blocked", err)
			}
			store, err := resourcetask.NewStore(f.db)
			if err != nil {
				t.Fatal(err)
			}
			tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "readiness-worker", Limit: 1, LeaseDuration: time.Minute})
			if err != nil || len(tasks) != 1 {
				t.Fatal("task claim", err)
			}
			if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_channel SET provider_plugin_id='idc-zjmfcloud' WHERE channel_no='channel-main'`); err != nil {
				t.Fatal(err)
			}
			route := serveroperationresourcecap.CapabilityRoute{ServiceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", PoolNo: "pool-main"}
			view, err := f.source.GetAccessories(f.ctx, route)
			if err != nil || len(view.Items) != 1 || view.Items[0].State != serveroperationresourcecap.AccessoryState(resourcecap.AccessoryStateUnknown) || slices.Contains(view.Actions, action) || !slices.Contains(view.Actions, deleteAction) {
				t.Fatal("old synthetic readiness survived projection", err)
			}
			if err = f.source.ValidateParameters(f.ctx, route, action, params); err == nil {
				t.Fatal("restoration validated with stale ready flag")
			}
			command.IdempotencyKey = "new-unverified-restore"
			if _, err = f.service.Create(f.ctx, command); err == nil {
				t.Fatal("new restore used unverified readiness")
			}
			gate, err := NewWriteAdmission(f.db, f.customers, f.source, f.service.objects)
			if err != nil {
				t.Fatal(err)
			}
			writes := 0
			task := tasks[0]
			if err = gate(f.ctx, task, resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: capability, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters)}, f.provider, func(context.Context) error { writes++; return nil }); err == nil || writes != 0 {
				t.Fatal("queued restoration reached supplier")
			}
			// Delete is justified by fresh unique object ownership, never by readiness.
			unknown, err := directory.Publish(f.ctx, accessorydirectory.PublishInput{MappingNo: "mapping-1", LocalResourceNo: route.ServiceNo, ChannelNo: route.ChannelNo, ExternalResourceRef: "external-1", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: time.Now().UTC(), ExpiresAt: now.Add(time.Minute), Items: []resourcecap.AccessoryItem{{Kind: kind, State: resourcecap.AccessoryStateUnknown, ExternalRef: "77", Data: json.RawMessage(`{}`)}}}})
			if err != nil {
				t.Fatal(err)
			}
			deleteParams, err := json.Marshal(map[string]string{"objectNo": unknown.Items[0].ObjectNo})
			if err != nil {
				t.Fatal(err)
			}
			if err = f.source.ValidateParameters(f.ctx, route, deleteAction, deleteParams); err != nil {
				t.Fatal("deletion confused readiness with identity", err)
			}
		})
	}
}
