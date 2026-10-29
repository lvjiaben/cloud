// Native accessory operations must translate owned local choices and reject
// choices that disappear between enqueue and dispatch.
package customeroperations

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

func TestNativeAccessoryOwnershipTranslationAndQueuedRecheckPostgreSQL(t *testing.T) {
	cases := []struct {
		action                         string
		capability                     resourcecap.CapabilityCode
		kind                           resourcecap.AccessoryKind
		state                          resourcecap.AccessoryState
		input, translated, constraints string
	}{
		{"security_group_attach", resourcecap.CapabilitySecurityGroup, resourcecap.AccessoryKindSecurityGroup, resourcecap.AccessoryStateAvailable, `{"objectNo":"OBJECT"}`, `{"action":"attach","security_group_ref":"101"}`, `[{"name":"action","required":true,"allowedValues":["attach","detach"]},{"name":"security_group_ref","minimum":1}]`},
		{"security_group_detach", resourcecap.CapabilitySecurityGroup, resourcecap.AccessoryKindSecurityGroup, resourcecap.AccessoryStateAttached, `{}`, `{"action":"detach"}`, `[{"name":"action","required":true,"allowedValues":["attach","detach"]},{"name":"security_group_ref","minimum":1}]`},
		{"nat_create", resourcecap.CapabilityNAT, resourcecap.AccessoryKindNAT, resourcecap.AccessoryStateActive, `{"name":"SSH","externalPort":2222,"internalPort":22,"protocol":"tcp"}`, `{"action":"create","name":"SSH","external_port":2222,"internal_port":22,"protocol":"tcp"}`, `[{"name":"action","required":true,"allowedValues":["create","delete"]},{"name":"name"},{"name":"external_port","minimum":1,"maximum":65535},{"name":"internal_port","minimum":1,"maximum":65535},{"name":"protocol","allowedValues":["tcp","udp","tcp_udp"]},{"name":"nat_ref","minimum":1}]`},
		{"nat_delete", resourcecap.CapabilityNAT, resourcecap.AccessoryKindNAT, resourcecap.AccessoryStateActive, `{"objectNo":"OBJECT"}`, `{"action":"delete","nat_ref":"101"}`, `[{"name":"action","required":true,"allowedValues":["create","delete"]},{"name":"nat_ref","minimum":1}]`},
		{"virtual_media_mount", resourcecap.CapabilityVirtualMedia, resourcecap.AccessoryKindVirtualMedia, resourcecap.AccessoryStateAvailable, `{"objectNo":"OBJECT"}`, `{"action":"mount","image_ref":"101"}`, `[{"name":"action","required":true,"allowedValues":["mount","unmount"]},{"name":"image_ref","minimum":1}]`},
		{"virtual_media_unmount", resourcecap.CapabilityVirtualMedia, resourcecap.AccessoryKindVirtualMedia, resourcecap.AccessoryStateMounted, `{}`, `{"action":"unmount"}`, `[{"name":"action","required":true,"allowedValues":["mount","unmount"]},{"name":"image_ref","minimum":1}]`},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			f := newManagementFixture(t)
			if _, err := f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_capability_snapshot_item SET support_code='conditional',constraints=?::jsonb WHERE capability_code=?`, tc.constraints, tc.capability); err != nil {
				t.Fatal(err)
			}
			directory, err := accessorydirectory.New(f.db)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			published, err := directory.Publish(f.ctx, accessorydirectory.PublishInput{MappingNo: "mapping-1", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", ExternalResourceRef: "external-1", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(time.Minute), Items: []resourcecap.AccessoryItem{{Kind: tc.kind, State: tc.state, ExternalRef: "101", DisplayName: "Owned choice", Data: json.RawMessage(`{}`)}}}})
			if err != nil {
				t.Fatal(err)
			}
			target, targetErr := f.service.CommercialTarget(f.ctx, f.serverNo)
			if targetErr != nil || target.LocalResourceNo != "SRV_"+strings.Repeat("1", 32) || target.EntitlementNo != "ent_"+strings.Repeat("1", 32) || target.RuntimeStatus != "running" || target.SnapshotNo != published.SnapshotNo {
				t.Fatalf("commercial context confused owner identities: %+v %v", target, targetErr)
			}
			command := Command{ServerNo: f.serverNo, Action: tc.action, IdempotencyKey: "accessory-1", Confirmed: true, Parameters: json.RawMessage(strings.ReplaceAll(tc.input, "OBJECT", published.Items[0].ObjectNo))}
			f.customers.customer = "another-customer"
			if _, err = f.service.CommercialTarget(f.ctx, f.serverNo); err == nil {
				t.Fatal("foreign commercial target disclosed")
			}
			if _, err = f.service.Create(f.ctx, command); err == nil {
				t.Fatal("foreign customer admitted")
			}
			f.customers.customer = "customer-a"
			if _, err = f.service.Create(f.ctx, command); err != nil {
				t.Fatal(err)
			}
			store, err := resourcetask.NewStore(f.db)
			if err != nil {
				t.Fatal(err)
			}
			tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "accessory-test", Limit: 1, LeaseDuration: time.Minute})
			if err != nil || len(tasks) != 1 {
				t.Fatalf("claim: %v", err)
			}
			task := tasks[0]
			got, err := normalizeTranslated([]byte(task.SanitizedParameters))
			if err != nil {
				t.Fatal(err)
			}
			want, err := normalizeTranslated([]byte(tc.translated))
			if err != nil || string(got) != string(want) {
				t.Fatalf("wrong translated choice: %s", got)
			}
			gate, err := NewWriteAdmission(f.db, f.customers, f.source, f.service.objects)
			if err != nil {
				t.Fatal(err)
			}
			request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: tc.capability, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters)}
			writes := 0
			invoke := func(context.Context) error { writes++; return nil }
			if err = gate(f.ctx, task, request, f.provider, invoke); err != nil || writes != 1 {
				t.Fatalf("owned dispatch: %v", err)
			}
			if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_accessory_snapshot SET observed_at=CURRENT_TIMESTAMP-INTERVAL '2 minutes',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second'`); err != nil {
				t.Fatal(err)
			}
			if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 1 {
				t.Fatal("expired directory reached provider")
			}
		})
	}
}
