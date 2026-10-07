package renewalsource

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serveroperationsource"
)

type serverOperationOutputSecretSource struct{}
type serverOperationOutputAccessor struct{}

func (serverOperationOutputSecretSource) Accessor(secretcap.Scope) (secretcap.Accessor, error) {
	return serverOperationOutputAccessor{}, nil
}
func (serverOperationOutputSecretSource) Manager(secretcap.Scope) (secretcap.Manager, error) {
	return nil, errors.New("manager is not used by output test")
}
func (serverOperationOutputAccessor) Access(_ context.Context, request secretcap.AccessRequest,
	consume secretcap.Consumer) error {
	if request.Handle.OpaqueRef() != "secret-console-ticket-1" {
		return errors.New("unexpected output handle")
	}
	return consume([]byte(`{"vnc_url_https":"https://console.example.test/session","token":"ticket"}`))
}

func TestServerOperationSourcePostgreSQLFreezesNormalAction(t *testing.T) {
	ctx, database := newRenewalSourceDatabase(t)
	seedRenewalSourceRoute(t, ctx, database)
	insertServerOperationCapabilitySnapshot(t, ctx, database, "capability-operation-1")
	insertServerOperationImageSnapshot(t, ctx, database)
	service, err := serveroperationsource.New(database)
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := service.ListCapabilities(ctx, []serveroperationresourcecap.CapabilityRoute{{
		ServiceNo: "service-renewal-1", ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1",
	}})
	if err != nil || len(capabilities) != 1 || len(capabilities[0].Actions) != 2 ||
		capabilities[0].ObservedAt.IsZero() {
		t.Fatalf("capabilities=%+v err=%v", capabilities, err)
	}
	route := serveroperationresourcecap.CapabilityRoute{
		ServiceNo: "service-renewal-1", ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1",
	}
	images, err := service.GetReinstallImages(ctx, route)
	if err != nil || images == nil || images.ServiceNo != route.ServiceNo || len(images.Images) != 2 ||
		images.Images[0].Name != "Debian 12" || !images.Images[0].Selectable ||
		images.Images[1].Name != "Unknown image" || images.Images[1].Selectable || images.ObservedAt.IsZero() {
		t.Fatalf("reinstall images=%+v err=%v", images, err)
	}
	if err = service.ValidateParameters(ctx, route, serveroperationresourcecap.ActionReboot,
		json.RawMessage(`{"mode":"hard"}`)); err != nil {
		t.Fatalf("valid operation parameters rejected: %v", err)
	}
	if err = service.ValidateParameters(ctx, route, serveroperationresourcecap.ActionReboot,
		json.RawMessage(`{"mode":"invalid"}`)); !errors.Is(err, serveroperationresourcecap.ErrCapabilityUnavailable) {
		t.Fatalf("invalid operation parameters returned %v", err)
	}
	if err = service.ValidateParameters(ctx, route, serveroperationresourcecap.ActionConsole,
		json.RawMessage(`{"unexpected":true}`)); !errors.Is(err, serveroperationresourcecap.ErrCapabilityUnavailable) {
		t.Fatalf("unexpected supported-operation parameters returned %v", err)
	}
	request := serveroperationresourcecap.TaskRequest{
		OperationNo: "SOP_0123456789abcdef0123456789abcdef", ServiceNo: "service-renewal-1",
		ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1",
		Action: serveroperationresourcecap.ActionReboot, ExpectedServiceVersion: 3,
		Parameters: json.RawMessage(`{"mode":"graceful"}`),
	}
	request.RequestDigest, err = serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateTask(ctx, request)
	if err != nil || created == nil || created.Reused || created.Status != serveroperationresourcecap.TaskStatusPending {
		t.Fatalf("create task=%+v err=%v", created, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_capability_snapshot
SET observed_at = CURRENT_TIMESTAMP - INTERVAL '2 days'
WHERE snapshot_no = 'capability-operation-1'`); err != nil {
		t.Fatal(err)
	}
	staleCapabilities, err := service.ListCapabilities(ctx, []serveroperationresourcecap.CapabilityRoute{{
		ServiceNo: "service-renewal-1", ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1",
	}})
	if err != nil || len(staleCapabilities) != 1 || len(staleCapabilities[0].Actions) != 0 {
		t.Fatalf("stale capabilities=%+v err=%v", staleCapabilities, err)
	}
	if err = service.ValidateParameters(ctx, route, serveroperationresourcecap.ActionReboot,
		json.RawMessage(`{"mode":"hard"}`)); !errors.Is(err, serveroperationresourcecap.ErrCapabilityUnavailable) {
		t.Fatalf("stale operation parameters returned %v", err)
	}
	replayed, err := service.CreateTask(ctx, request)
	if err != nil || replayed == nil || !replayed.Reused || replayed.ProviderTaskNo != created.ProviderTaskNo {
		t.Fatalf("replay task=%+v err=%v", replayed, err)
	}
	changed := request
	changed.Parameters = json.RawMessage(`{"mode":"hard"}`)
	changed.RequestDigest, _ = serveroperationresourcecap.TaskRequestDigest(changed)
	if _, err = service.CreateTask(ctx, changed); !errors.Is(err, serveroperationresourcecap.ErrTaskConflict) {
		t.Fatalf("changed replay returned %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code = 'succeeded', finished_at = CURRENT_TIMESTAMP,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE task_no = ?`, created.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	completed, err := service.ReadTask(ctx, serveroperationresourcecap.TaskReadRequest{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
	})
	if err != nil || completed == nil || completed.Status != serveroperationresourcecap.TaskStatusSucceeded || completed.CompletedAt.IsZero() {
		t.Fatalf("read task=%+v err=%v", completed, err)
	}
	consoleRequest := serveroperationresourcecap.TaskRequest{
		OperationNo: "SOP_console_0123456789abcdef0123456", ServiceNo: "service-renewal-1",
		ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1",
		Action: serveroperationresourcecap.ActionConsole, ExpectedServiceVersion: 4,
		Parameters: json.RawMessage(`{}`),
	}
	consoleRequest.RequestDigest, err = serveroperationresourcecap.TaskRequestDigest(consoleRequest)
	if err != nil {
		t.Fatal(err)
	}
	// Restore the same immutable capability observation time only for this
	// isolated fixture; production snapshots are append-only publications.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot
SET observed_at = CURRENT_TIMESTAMP WHERE snapshot_no = 'capability-operation-1'`); err != nil {
		t.Fatal(err)
	}
	consoleTask, err := service.CreateTask(ctx, consoleRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task
SET status_code = 'succeeded', finished_at = CURRENT_TIMESTAMP,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE task_no = ?`, consoleTask.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_task_secret_binding
    (binding_no, task_no, binding_name, binding_direction, secret_ref,
     output_policy, expires_at)
VALUES ('binding-console-1', ?, 'consoleTicket', 'output',
        'secret-console-ticket-1', 'short_lived_ticket', CURRENT_TIMESTAMP + INTERVAL '5 minutes')`,
		consoleTask.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	consoleCompleted, err := service.ReadTask(ctx, serveroperationresourcecap.TaskReadRequest{
		ProviderTaskNo: consoleTask.ProviderTaskNo, Request: consoleRequest,
	})
	if err != nil || consoleCompleted == nil || len(consoleCompleted.SecretOutputs) != 1 ||
		consoleCompleted.SecretOutputs[0].Name != "consoleTicket" ||
		consoleCompleted.SecretOutputs[0].Handle.OpaqueRef() != "secret-console-ticket-1" {
		t.Fatalf("read console task=%+v err=%v", consoleCompleted, err)
	}
	delivery, err := serveroperationsource.NewWithSecrets(database, serverOperationOutputSecretSource{})
	if err != nil {
		t.Fatal(err)
	}
	var payload []byte
	err = delivery.RedeemOutput(ctx, serveroperationresourcecap.ConfidentialOutputRequest{
		ProviderTaskNo: consoleTask.ProviderTaskNo, ChannelNo: "channel-renewal-1",
		Name: "consoleTicket", Policy: serveroperationresourcecap.SecretOutputPolicyShortLivedTicket,
		Handle: consoleCompleted.SecretOutputs[0].Handle, ExpiresAt: consoleCompleted.SecretOutputs[0].ExpiresAt,
	}, func(value []byte) error {
		payload = append(payload, value...)
		return nil
	})
	if err != nil || string(payload) != `{"vnc_url_https":"https://console.example.test/session","token":"ticket"}` {
		t.Fatalf("redeem console payload=%s err=%v", payload, err)
	}
	clear(payload)
	if err = delivery.RedeemOutput(ctx, serveroperationresourcecap.ConfidentialOutputRequest{
		ProviderTaskNo: consoleTask.ProviderTaskNo, ChannelNo: "channel-renewal-1",
		Name: "consoleTicket", Policy: serveroperationresourcecap.SecretOutputPolicyShortLivedTicket,
		Handle: consoleCompleted.SecretOutputs[0].Handle, ExpiresAt: consoleCompleted.SecretOutputs[0].ExpiresAt,
	}, func([]byte) error { return nil }); !errors.Is(err, serveroperationresourcecap.ErrOutputConsumed) {
		t.Fatalf("replayed console output returned %v", err)
	}
	// Re-arm the isolated fixture with an already expired timestamp so this test
	// proves database time rejects the ticket before revealing its secret.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task_secret_binding
SET status_code='active',revealed_at=NULL,expires_at=CURRENT_TIMESTAMP - INTERVAL '1 minute'
WHERE task_no=? AND binding_name='consoleTicket'`, consoleTask.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	expiredValue, err := database.GetValue(ctx, `SELECT expires_at
FROM plugin_idc_resource_task_secret_binding
WHERE task_no=? AND binding_name='consoleTicket'`, consoleTask.ProviderTaskNo)
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := expiredValue.Time()
	if err = delivery.RedeemOutput(ctx, serveroperationresourcecap.ConfidentialOutputRequest{
		ProviderTaskNo: consoleTask.ProviderTaskNo, ChannelNo: "channel-renewal-1",
		Name: "consoleTicket", Policy: serveroperationresourcecap.SecretOutputPolicyShortLivedTicket,
		Handle: consoleCompleted.SecretOutputs[0].Handle, ExpiresAt: expiredAt.UTC(),
	}, func([]byte) error { return nil }); !errors.Is(err, serveroperationresourcecap.ErrOutputUnavailable) {
		t.Fatalf("expired console output returned %v", err)
	}
	expiredStatusValue, err := database.GetValue(ctx, `SELECT status_code
FROM plugin_idc_resource_task_secret_binding
WHERE task_no=? AND binding_name='consoleTicket'`, consoleTask.ProviderTaskNo)
	if err != nil {
		t.Fatal(err)
	}
	expiredStatus := expiredStatusValue.String()
	if expiredStatus != "active" {
		t.Fatalf("expired output was consumed with status %q", expiredStatus)
	}
}

func insertServerOperationImageSnapshot(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provider_capacity_observation
 (observation_no,channel_no,capacity_kind,status_code,observation_fingerprint,item_count,observed_at,completed_at)
VALUES ('images-operation-1','channel-renewal-1','image','complete',repeat('a',64),2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_resource_provider_capacity_observation_item
 (observation_no,capacity_kind,external_ref,projection,observed_at)
VALUES
 ('images-operation-1','image','101','{"name":"Debian 12","group":"Linux","status":1,"os_type":"linux","arch":"amd64","boot_mode":"uefi"}'::jsonb,CURRENT_TIMESTAMP),
 ('images-operation-1','image','102','{"name":"Unknown image","group":"Other","status":"mystery","os_type":"other","arch":"amd64","boot_mode":"bios"}'::jsonb,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
}

func insertServerOperationCapabilitySnapshot(t *testing.T, ctx context.Context, database gdb.DB, snapshotNo string) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_capability_snapshot
    (snapshot_no, channel_no, adapter_version, status_code, item_count, observed_at, completed_at)
VALUES (?, 'channel-renewal-1', 'test-v1', 'complete', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, snapshotNo, resourcecap.MaximumCapabilityCount); err != nil {
		t.Fatal(err)
	}
	for _, capability := range resourcecap.SupportedCapabilityCodes() {
		support, constraints, reason := "unsupported", `[]`, "not used by fixture"
		if capability == resourcecap.CapabilityReboot {
			support, constraints, reason = "conditional", `[{"name":"mode","required":false,"allowedValues":["graceful","hard"]}]`, ""
		} else if capability == resourcecap.CapabilityConsole {
			support, constraints, reason = "supported", `[]`, ""
		}
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_capability_snapshot_item
    (snapshot_no, capability_code, support_code, constraints, unsupported_reason)
VALUES (?, ?, ?, CAST(? AS JSONB), ?)`, snapshotNo, capability, support, constraints, reason); err != nil {
			t.Fatalf("insert capability %s: %v", capability, err)
		}
	}
}
