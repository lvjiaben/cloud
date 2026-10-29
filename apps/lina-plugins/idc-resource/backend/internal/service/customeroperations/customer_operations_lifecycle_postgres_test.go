// This file checks lifecycle routing against independently numbered native resources.
package customeroperations

import (
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/lifecyclesource"
)

func TestNativeEntitlementLifecycleRoutePostgreSQL(t *testing.T) {
	ctx, database := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "1", "active", "succeeded")
	if _, err := database.Exec(ctx, `DELETE FROM plugin_idc_resource_capability_snapshot_item;
UPDATE plugin_idc_resource_capability_snapshot SET item_count=29;
UPDATE plugin_idc_resource_channel SET provider_runtime_type='dynamic',provider_contract_version='v1',provider_release_version='1.0.0',provider_release_digest=repeat('a',64),provider_declaration_digest=repeat('b',64);
INSERT INTO plugin_idc_resource_interface_channel_binding(interface_instance_no,channel_no)
VALUES('pfi_11111111111111111111111111111111','channel-main');`); err != nil {
		t.Fatal(err)
	}
	for _, capability := range resourcecap.SupportedCapabilityCodes() {
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_capability_snapshot_item(snapshot_no,capability_code,support_code,constraints,unsupported_reason) VALUES('capability-main',?,'supported','[]','')`, capability); err != nil {
			t.Fatal(err)
		}
	}
	source, err := lifecyclesource.New(database)
	if err != nil {
		t.Fatal(err)
	}
	request := lifecycleresourcecap.TaskRequest{LifecycleTaskNo: "lifecycle-native-route", ServiceNo: "ent_" + strings.Repeat("1", 32), CustomerNo: "customer-a", Action: lifecycleresourcecap.ActionSuspend, ExpectedServiceVersion: 2, ExpectedExpiresAt: time.Now().UTC(), IdempotencyDigest: strings.Repeat("a", 64)}
	if _, err = source.CreateTask(ctx, request); err == nil {
		t.Fatal("dynamic lifecycle task accepted missing configuration")
	}
	revision := "rci_" + strings.Repeat("2", 32)
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_interface_channel_binding SET current_configuration_revision=?`, revision); err != nil {
		t.Fatal(err)
	}
	foreign := request
	foreign.CustomerNo = "customer-b"
	if _, err = source.CreateTask(ctx, foreign); err == nil {
		t.Fatal("foreign lifecycle request accepted")
	}
	created, err := source.CreateTask(ctx, request)
	if err != nil || created == nil || created.Status != lifecycleresourcecap.TaskStatusPending {
		t.Fatalf("create native lifecycle task: %+v err=%v", created, err)
	}
	var stored struct{ LocalResourceNo, Revision string }
	if err = database.GetScan(ctx, &stored, `SELECT local_resource_no,owner_metadata->>'interfaceConfigurationRevision' AS revision FROM plugin_idc_resource_task WHERE task_no=?`, created.ProviderTaskNo); err != nil || stored.LocalResourceNo != "SRV_"+strings.Repeat("1", 32) || stored.Revision != revision {
		t.Fatalf("lifecycle lost exact resource/configuration: %+v err=%v", stored, err)
	}
	replay, err := source.CreateTask(ctx, request)
	if err != nil || replay == nil || !replay.Reused || replay.ProviderTaskNo != created.ProviderTaskNo {
		t.Fatalf("lifecycle replay duplicated task: %+v err=%v", replay, err)
	}
	if read, readErr := source.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{ProviderTaskNo: created.ProviderTaskNo, Request: request}); readErr != nil || read == nil || read.ProviderTaskNo != created.ProviderTaskNo {
		t.Fatalf("read normalized lifecycle task: %+v err=%v", read, readErr)
	}
}
