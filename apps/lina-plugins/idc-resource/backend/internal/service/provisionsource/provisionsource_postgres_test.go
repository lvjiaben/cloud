package provisionsource

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	ownercomposition "lina-plugin-idc-resource/backend/internal/composition"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"lina-plugin-idc-resource/backend/internal/service/serverownership"
)

const provisionSourceDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

const provisionInterfaceInstanceNo = "pfi_11111111111111111111111111111111"

func TestProvisionSourcePostgreSQLResolvesInterfaceInsideOwnerChannel(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_pool
    (pool_no,channel_no,display_name,resource_kind,status_code,routing_priority,routing_weight,selector)
VALUES ('pool-incompatible-1','channel-provision-1','Incompatible Pool',
        'virtual_server','enabled',1,100,
        '{"provision":{"provider":"zjmfcloud-v1","areas":{"other":"9"},"nodes":{"premium":"2"},"node_groups":{},"images":{"ubuntu-2204":"3"},"defaults":{}}}'::jsonb)`); err != nil {
		t.Fatalf("seed incompatible interface pool: %v", err)
	}
	secrets := newProvisionSecretSource(t)
	service, err := New(database, secrets, principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	created, err := service.CreateTask(ctx, request)
	if err != nil || created == nil || created.ChannelNo != "channel-provision-1" {
		t.Fatalf("create frozen provision task=%+v err=%v", created, err)
	}
	var row struct {
		PoolNo          string `orm:"pool_no"`
		LocalResourceNo string `orm:"local_resource_no"`
		Parameters      string `orm:"parameters"`
	}
	if err = database.GetScan(ctx, &row, `
SELECT pool_no,local_resource_no,sanitized_parameters::TEXT AS parameters
FROM plugin_idc_resource_task WHERE task_no=?`, created.ProviderTaskNo); err != nil {
		t.Fatalf("load frozen provision task: %v", err)
	}
	if row.PoolNo != "pool-provision-1" || !strings.HasPrefix(row.LocalResourceNo, "SRV_") ||
		strings.Contains(row.Parameters, "operating_system") ||
		!strings.Contains(row.Parameters, `"os": "3"`) {
		t.Fatalf("unexpected owner-routed task: %+v", row)
	}
	replayed, err := service.CreateTask(ctx, request)
	if err != nil || replayed == nil || !replayed.Reused ||
		replayed.ProviderTaskNo != created.ProviderTaskNo || secrets.createCalls() != 1 {
		t.Fatalf("replay frozen provision task=%+v calls=%d err=%v", replayed, secrets.createCalls(), err)
	}
	changed := request
	changed.CustomerConfiguration = json.RawMessage(`{"operating_system":"debian-12"}`)
	changed.RequestDigest, err = provisionresourcecap.TaskRequestDigest(changed)
	if err != nil {
		t.Fatalf("digest changed frozen request: %v", err)
	}
	if _, err = service.CreateTask(ctx, changed); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
		t.Fatalf("changed frozen replay returned %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code='succeeded',finished_at=CURRENT_TIMESTAMP,
    failure_code='',result_unknown=FALSE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE task_no=?`, created.ProviderTaskNo); err != nil {
		t.Fatalf("complete frozen provision task: %v", err)
	}
	succeeded, err := service.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
	})
	if err != nil || succeeded == nil || succeeded.Status != provisionresourcecap.TaskStatusSucceeded {
		t.Fatalf("read succeeded frozen provision task=%+v err=%v", succeeded, err)
	}
	if err = service.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
		Status:             provisionresourcecap.OutcomeStatusSucceeded,
		TaskEvidenceDigest: succeeded.EvidenceDigest,
		CoreEvidenceDigest: strings.Repeat("e", 64),
	}); err != nil {
		t.Fatalf("report frozen provision outcome: %v", err)
	}
	assertProvisionCredentialState(t, ctx, database, created.ProviderTaskNo, "revealable", 1)
}

// Provider failure codes use the resource owner's casing; the core contract
// requires canonical lower case. Real failed and unknown tasks must remain readable.
func TestProvisionSourcePostgreSQLReadsUppercaseProviderFailure(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	service, err := New(database, newProvisionSecretSource(t), principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	created, err := service.CreateTask(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		status   string
		want     provisionresourcecap.TaskStatus
		finished bool
	}{
		{string(resourcetask.StatusFailed), provisionresourcecap.TaskStatusFailed, true},
		{string(resourcetask.StatusResultUnknown), provisionresourcecap.TaskStatusResultUnknown, false},
		{string(resourcetask.StatusManualTakeover), provisionresourcecap.TaskStatusManualReview, true},
	} {
		t.Run(test.status, func(t *testing.T) {
			_, err := database.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code=?,failure_code='UPSTREAM_RESULT_UNKNOWN',result_unknown=?,finished_at=CASE WHEN ? THEN CURRENT_TIMESTAMP ELSE NULL END,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=?`, test.status, test.status == string(resourcetask.StatusResultUnknown), test.finished, created.ProviderTaskNo)
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: created.ProviderTaskNo, Request: request})
			if err != nil || result == nil || result.Status != test.want || result.FailureCode != "upstream_result_unknown" {
				t.Fatalf("task projection status=%s result=%+v err=%v", test.status, result, err)
			}
			if err := provisionresourcecap.ValidateTask(*result); err != nil {
				t.Fatal(err)
			}
		})
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_task`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("reads must not create tasks: count=%v err=%v", count, err)
	}
}

func TestProvisionSourcePostgreSQLRejectsLegacySourceRuntimeBeforeProviderInvocation(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_resource_channel
SET provider_runtime_type='source',provider_contract_version='',provider_release_version='',
    provider_release_digest='',provider_declaration_digest=''
WHERE channel_no='channel-provision-1'`); err != nil {
		t.Fatalf("change fixture to legacy source runtime: %v", err)
	}
	calls := 0
	secrets := newProvisionSecretSource(t)
	service, err := New(database, secrets, principalReady, nil, nil, provisionManagedBuilder{calls: &calls})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateTask(ctx, frozenProvisionSourceRequest(t)); !errors.Is(err, provisionresourcecap.ErrCapabilityUnavailable) {
		t.Fatalf("legacy source runtime returned %v", err)
	}
	count, countErr := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_task`)
	if countErr != nil || count.Int() != 0 || calls != 0 || secrets.createCalls() != 0 {
		t.Fatalf("legacy source runtime crossed Provider boundary: tasks=%d builder_calls=%d secret_calls=%d err=%v", count.Int(), calls, secrets.createCalls(), countErr)
	}
}

func TestPaidServerOwnerRunsDynamicProviderAndProjectsEntitlementPostgreSQL(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	secrets := newProvisionSecretSource(t)
	source, err := New(database, secrets, principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	created, err := source.CreateTask(ctx, request)
	if err != nil || created == nil {
		t.Fatalf("create resource-owner task=%+v err=%v", created, err)
	}
	// Historical uncorrected tasks did not carry the paid-interface field.
	// The exact original-channel binding must remain valid without rewriting
	// their persisted metadata or requiring a provider call to migrate it.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task SET owner_metadata=owner_metadata-'purchasedInterfaceInstanceNo' WHERE task_no=?`, created.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatal(err)
	}
	channels, err := channelconfig.NewReader(database)
	if err != nil {
		t.Fatal(err)
	}
	registrar := &provisionDynamicRegistrar{}
	runtime, err := ownercomposition.NewProviderRuntime(
		nil, secrets, channels,
		func(context.Context, channelconfig.Configuration) (resourcecap.ProviderEnv, func(), error) {
			t.Fatal("dynamic provider must not construct a source-plugin environment")
			return resourcecap.ProviderEnv{}, nil, nil
		},
		ownercomposition.WithDynamicBusinessExtensions(registrar, func(
			_ context.Context,
			configuration channelconfig.Configuration,
			method string,
			operationSecrets []resourcecap.SecretBinding,
			_ ...func(resourcecap.Evidence),
		) (pluginhost.ConfidentialOutboundHandler, func(), error) {
			if configuration.Config.ChannelRef != "channel-provision-1" ||
				configuration.ReleaseVersion != "v0.1.0" ||
				method != resourcecap.DynamicProviderMethodExecute || len(operationSecrets) != 1 {
				t.Fatalf("unexpected frozen dynamic provider scope: config=%+v method=%s secrets=%d", configuration, method, len(operationSecrets))
			}
			return func(_ context.Context, outbound *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
				registrar.outboundCalls++
				if outbound == nil || bytes.Contains(outbound.Payload, []byte("provider.invalid")) ||
					bytes.Contains(outbound.Payload, []byte("test-secret-")) {
					return nil, errors.New("unsafe dynamic outbound payload")
				}
				return &pluginhost.ConfidentialOutboundResult{Payload: json.RawMessage(`{"StatusCode":200,"Data":{"accepted":true}}`)}, nil
			}, func() {}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	health := &provisionHealthRecorder{}
	executor, err := resourcetask.NewExecutor(
		store, runtime.ConstructRevisionSession, store.ResolveResources, health,
		func(ctx context.Context, _ resourcetask.ClaimedTask, _ resourcecap.OperationRequest, _ resourcecap.Provider, invoke func(context.Context) error) error {
			return invoke(ctx)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := executor.RunWrites(ctx, resourcetask.ClaimInput{
		WorkerID: "composition-worker", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || report.Claimed != 1 || report.Persisted != 1 || registrar.invocations != 1 || registrar.outboundCalls != 1 || health.records != 1 {
		t.Fatalf("dynamic provider execution report=%+v invocations=%d outbound=%d health=%d err=%v", report, registrar.invocations, registrar.outboundCalls, health.records, err)
	}
	succeeded, err := source.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
	})
	if err != nil || succeeded == nil || succeeded.Status != provisionresourcecap.TaskStatusSucceeded {
		t.Fatalf("read composed resource task=%+v err=%v", succeeded, err)
	}
	if err = source.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{
		ProviderTaskNo: created.ProviderTaskNo, Request: request,
		Status:             provisionresourcecap.OutcomeStatusSucceeded,
		TaskEvidenceDigest: succeeded.EvidenceDigest,
		CoreEvidenceDigest: strings.Repeat("e", 64),
	}); err != nil {
		t.Fatalf("persist composed core outcome: %v", err)
	}
	sink, err := serverownership.NewEntitlementCreatedSink(database)
	if err != nil {
		t.Fatal(err)
	}
	event := entitlementcreatedcap.Event{
		Version: entitlementcreatedcap.ContractVersion,
		EventNo: "event-composed-server-1", EntitlementNo: "ent_22222222222222222222222222222222",
		IntentNo: request.OperationNo, CustomerNo: request.CustomerNo,
		ProductNo: request.Fulfillment.ProductNo, ProductVersionNo: request.Fulfillment.ProductVersionNo,
		ProductNameZhCN: "组合测试云服务器", ProductNameEnUS: "Composed cloud server",
		ConfigurationSnapshotNo: request.ConfigurationSnapshotNo,
		InterfaceInstanceNo:     request.InterfaceInstanceNo,
		PluginID:                "idc-zjmfcloud", PluginReleaseVersion: "v0.1.0",
		ContractVersion: "v1", DeclarationDigest: strings.Repeat("b", 64),
		OwnerID: serverownership.EntitlementOwnerID,
	}
	receipt, err := sink.Deliver(ctx, event)
	if err != nil || receipt == nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) {
		t.Fatalf("project composed server entitlement receipt=%+v err=%v", receipt, err)
	}
	var evidence struct {
		TaskStatus, MappingStatus, ExternalResourceRef, CustomerNo string
		OutcomeReceipts, EntitlementRows                           int
	}
	if err = database.GetScan(ctx, &evidence, `SELECT task.status_code task_status,mapping.status_code mapping_status,
mapping.external_resource_ref,projection.customer_no,
(SELECT count(*) FROM plugin_idc_resource_provision_outcome_receipt WHERE task_no=task.task_no) outcome_receipts,
(SELECT count(*) FROM plugin_idc_resource_server_entitlement_projection WHERE entitlement_no=?) entitlement_rows
FROM plugin_idc_resource_task task
JOIN plugin_idc_resource_mapping mapping ON mapping.local_resource_no=task.local_resource_no AND mapping.channel_no=task.channel_no
JOIN plugin_idc_resource_server_entitlement_projection projection ON projection.mapping_no=mapping.mapping_no
WHERE task.task_no=?`, event.EntitlementNo, created.ProviderTaskNo); err != nil ||
		evidence.TaskStatus != "succeeded" || evidence.MappingStatus != "active" ||
		evidence.ExternalResourceRef != "upstream-composed-server-41" ||
		evidence.CustomerNo != request.CustomerNo || evidence.OutcomeReceipts != 1 || evidence.EntitlementRows != 1 {
		t.Fatalf("composed server evidence=%+v err=%v", evidence, err)
	}
}

type provisionDynamicRegistrar struct {
	registration  pluginhost.ConfidentialOutboundContextInput
	invocations   int
	outboundCalls int
}

type provisionHealthRecorder struct{ records int }

func (*provisionHealthRecorder) CheckWrites(context.Context, []string) (map[string]providerhealth.Gate, error) {
	return map[string]providerhealth.Gate{}, nil
}
func (*provisionHealthRecorder) CheckWrite(context.Context, string) (providerhealth.Gate, error) {
	return providerhealth.Gate{}, nil
}
func (health *provisionHealthRecorder) Record(context.Context, providerhealth.Observation) error {
	health.records++
	return nil
}

func (*provisionDynamicRegistrar) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, errors.New("unused")
}
func (*provisionDynamicRegistrar) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*provisionDynamicRegistrar) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*provisionDynamicRegistrar) WithNewBusinessAdmission(context.Context, pluginhost.BusinessExtensionAdmissionInput, capmodel.BusinessExtensionAdmissionAction) error {
	return errors.New("unused")
}
func (registrar *provisionDynamicRegistrar) WithConfidentialOutboundContext(
	ctx context.Context,
	input pluginhost.ConfidentialOutboundContextInput,
	action pluginhost.ConfidentialOutboundAction,
) error {
	registrar.registration = input
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "opaque-composition-context"})
}
func (registrar *provisionDynamicRegistrar) InvokeProviderMethod(
	ctx context.Context,
	input pluginhost.BusinessExtensionMethodInvokeInput,
) (*capmodel.BusinessExtensionMethodResult, error) {
	registrar.invocations++
	if registrar.registration.Handler == nil || input.RuntimeContext == nil ||
		input.RuntimeContext.ContextID != "opaque-composition-context" ||
		input.Method != resourcecap.DynamicProviderMethodExecute ||
		bytes.Contains(input.Payload, []byte("provider.invalid")) ||
		bytes.Contains(input.Payload, []byte("test-secret-")) {
		return nil, errors.New("unsafe or invalid isolated provider invocation")
	}
	if _, err := registrar.registration.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{
		Sequence: 1, Payload: append(json.RawMessage(nil), input.Payload...),
	}); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(resourcecap.OperationResult{
		Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: "upstream-composed-server-41",
	})
	return &capmodel.BusinessExtensionMethodResult{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: payload,
	}, err
}

func assertProvisionCredentialState(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	taskNo string,
	status string,
	receipts int,
) {
	t.Helper()
	var state struct {
		Status   string `orm:"status_code"`
		Receipts int    `orm:"receipts"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT credential.status_code,
       (SELECT COUNT(*) FROM plugin_idc_resource_provision_outcome_receipt
        WHERE task_no = credential.task_no) AS receipts
FROM plugin_idc_resource_provision_credential credential
WHERE credential.task_no = ?`, taskNo); err != nil {
		t.Fatalf("load provision credential state: %v", err)
	}
	if state.Status != status || state.Receipts != receipts {
		t.Fatalf("provision credential state=%+v want status=%s receipts=%d", state, status, receipts)
	}
}

func assertProvisionPersistence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	secrets *provisionSecretSource,
	taskNo string,
) {
	t.Helper()
	var row struct {
		Parameters, OwnerMetadata, TaskSecretRef, CredentialSecretRef string
		CredentialCount                                               int
	}
	err := database.GetScan(ctx, &row, `
SELECT task.sanitized_parameters::TEXT AS parameters,
       task.owner_metadata::TEXT AS owner_metadata,
       binding.secret_ref AS task_secret_ref,
       credential.secret_ref AS credential_secret_ref,
       COUNT(credential.id) OVER ()::INT AS credential_count
FROM plugin_idc_resource_task task
INNER JOIN plugin_idc_resource_task_secret_binding binding
  ON binding.task_no = task.task_no AND binding.binding_name = 'password'
INNER JOIN plugin_idc_resource_provision_credential credential
  ON credential.task_no = task.task_no
WHERE task.task_no = ?`, taskNo)
	if err != nil || row.CredentialCount != 1 || row.TaskSecretRef == "" ||
		row.TaskSecretRef != row.CredentialSecretRef {
		t.Fatalf("provision persistence=%+v err=%v", row, err)
	}
	if strings.Contains(row.Parameters, "customer-provision-1") ||
		strings.Contains(row.Parameters, "password") || strings.Contains(row.Parameters, "rootpass") ||
		!strings.Contains(row.Parameters, `"client": "88"`) {
		t.Fatalf("unsafe provision parameters: %s", row.Parameters)
	}
	if strings.Contains(row.OwnerMetadata, row.TaskSecretRef) || len(secrets.plaintext()) != 20 {
		t.Fatalf("secret leaked or plaintext was not generated: metadata=%s", row.OwnerMetadata)
	}
}

func seedProvisionRoute(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	selector := `{"provision":{"provider":"zjmfcloud-v1","areas":{"cn-south":"1"},"nodes":{"premium":"2"},"node_groups":{},"images":{"ubuntu-2204":"3"},"defaults":{"network_type":"normal","ip_group":"4","traffic_type":3,"node_priority":1}}}`
	principalDigest := strings.Repeat("a", 64)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no, provider_id, provider_plugin_id, provider_runtime_type,
     provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,
     display_name, endpoint, status_code)
VALUES ('channel-provision-1', 'zjmfcloud-v1', 'idc-zjmfcloud',
		'dynamic','v1','v0.1.0',repeat('a',64),repeat('b',64),
		'Provision Test', 'https://provider.invalid', 'enabled')`); err != nil {
		t.Fatalf("seed provision channel: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_interface_channel_binding
    (interface_instance_no,channel_no,binding_version,current_configuration_revision,
     product_plugin_id,product_plugin_release_version,product_contract_version,
     product_authorized_methods,product_release_digest,product_declaration_digest)
VALUES (?, 'channel-provision-1', 1, 'rci_11111111111111111111111111111111',
        'idc-zjmfcloud','v0.1.0','v1',ARRAY['provision','query'],repeat('a',64),repeat('b',64))`, provisionInterfaceInstanceNo); err != nil {
		t.Fatalf("seed provision interface binding: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_interface_configuration_revision
    (revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by)
VALUES ('rci_11111111111111111111111111111111',?,'channel-provision-1',
        'https://provider.invalid','{}'::jsonb,'{}'::jsonb,101)`, provisionInterfaceInstanceNo); err != nil {
		t.Fatalf("seed provision interface revision: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_pool
    (pool_no, channel_no, display_name, resource_kind, status_code, selector)
VALUES ('pool-provision-1', 'channel-provision-1', 'Provision Pool',
		'virtual_server', 'enabled', CAST(? AS JSONB))`, selector); err != nil {
		t.Fatalf("seed provision pool: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_principal_mapping
    (mapping_no, customer_no, channel_no, external_principal_ref,
     status_code, evidence_digest, observed_at)
VALUES ('principal-provision-1', 'customer-provision-1', 'channel-provision-1',
		'88', 'active', ?, CURRENT_TIMESTAMP)`, principalDigest); err != nil {
		t.Fatalf("seed provision principal: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_capability_snapshot
    (snapshot_no, channel_no, adapter_version, status_code, item_count,
     observed_at, completed_at)
VALUES ('capability-provision-1', 'channel-provision-1', 'zjmfcloud-admin-v1',
		'complete', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		resourcecap.MaximumCapabilityCount); err != nil {
		t.Fatalf("seed provision capability snapshot: %v", err)
	}
	constraints := `[
        {"name":"num","required":false,"allowedValues":["1"]},
        {"name":"cpu","required":true,"minimum":1},
        {"name":"memory","required":true,"minimum":128},
        {"name":"os","required":true},
        {"name":"client","required":true,"minimum":1},
        {"name":"hostname","required":true}
    ]`
	for _, capability := range resourcecap.SupportedCapabilityCodes() {
		support, itemConstraints, reason := "unsupported", `[]`, "unused by provision fixture"
		if capability == resourcecap.CapabilityProvision {
			support, itemConstraints, reason = "conditional", constraints, ""
		}
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_capability_snapshot_item
    (snapshot_no, capability_code, support_code, constraints, unsupported_reason)
VALUES ('capability-provision-1', ?, ?, CAST(? AS JSONB), ?)`,
			capability, support, itemConstraints, reason); err != nil {
			t.Fatalf("seed provision capability %s: %v", capability, err)
		}
	}
}

func frozenProvisionSourceRequest(t *testing.T) provisionresourcecap.TaskRequest {
	t.Helper()
	request := provisionresourcecap.TaskRequest{
		OperationNo: "intent-provision-1", CustomerNo: "customer-provision-1",
		ConfigurationSnapshotNo:        "configuration-provision-1",
		InterfaceInstanceNo:            provisionInterfaceInstanceNo,
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		ProductConfiguration: json.RawMessage(
			`{"region":"cn-south","line":"premium","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100,"upstream_product_id":"v10-product-41","upstream_region_option_id":"region-3"}`,
		),
		ProductConfigurationDigest: strings.Repeat("d", 64),
		CustomerConfiguration:      json.RawMessage(`{"operating_system":"ubuntu-2204"}`),
		Fulfillment: provisionresourcecap.FrozenFulfillmentIdentity{
			IntentNo: "intent-provision-1", OrderNo: "order-provision-1",
			OrderItemNo: "item-provision-1", ProductNo: "product-provision-1",
			ProductVersionNo: "version-provision-1", ConfigurationRevision: "revision-1",
			UnitSequence: 1, Quantity: 1,
		},
	}
	var err error
	request.RequestDigest, err = provisionresourcecap.TaskRequestDigest(request)
	if err != nil {
		t.Fatalf("digest frozen provision request: %v", err)
	}
	return request
}

type provisionManagedBuilder struct{ calls *int }

func (b provisionManagedBuilder) Build(
	ctx context.Context,
	channelNo string,
	revisionNo string,
	plan resourcecap.ServerProvisionPlan,
) (json.RawMessage, error) {
	if b.calls != nil {
		(*b.calls)++
	}
	if ctx == nil || ctx.Err() != nil || channelNo != "channel-provision-1" ||
		revisionNo != "rci_11111111111111111111111111111111" ||
		len(plan.ProductConfiguration) == 0 || len(plan.CustomerConfiguration) == 0 ||
		!strings.Contains(string(plan.Selector), `"cn-south"`) ||
		!strings.Contains(string(plan.ProductConfiguration), `"upstream_product_id":"v10-product-41"`) {
		return nil, errors.New("frozen dynamic interface invocation is unavailable")
	}
	return json.Marshal(map[string]any{
		"num": 1, "cpu": 2, "memory": 4096, "os": "3",
		"client": plan.ExternalPrincipalRef, "hostname": plan.Hostname,
	})
}

type provisionSecretSource struct {
	mutex         sync.Mutex
	results       map[string]secretcap.CreateResult
	plain         []byte
	callCount     int
	producerCount int
	revokeCount   int
}

func newProvisionSecretSource(t *testing.T) *provisionSecretSource {
	t.Helper()
	return &provisionSecretSource{results: make(map[string]secretcap.CreateResult)}
}

func (*provisionSecretSource) Accessor(secretcap.Scope) (secretcap.Accessor, error) {
	return provisionSecretAccessor{}, nil
}

func (s *provisionSecretSource) Manager(secretcap.Scope) (secretcap.Manager, error) { return s, nil }

func (s *provisionSecretSource) Create(
	_ context.Context,
	idempotencyKey string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.callCount++
	if result, exists := s.results[idempotencyKey]; exists {
		result.Reused = true
		return result, nil
	}
	s.producerCount++
	if err := produce(func(value []byte) error {
		s.plain = append([]byte(nil), value...)
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	digest := hex.EncodeToString([]byte(idempotencyKey))
	if len(digest) > 48 {
		digest = digest[:48]
	}
	handle, err := secretcap.NewHandle("test-secret-" + digest)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	result := secretcap.CreateResult{Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: time.Now().UTC()}}
	s.results[idempotencyKey] = result
	return result, nil
}

func (s *provisionSecretSource) RecoverCreate(
	_ context.Context,
	idempotencyKey string,
) (secretcap.CreateResult, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	result, exists := s.results[idempotencyKey]
	if !exists {
		return secretcap.CreateResult{}, errors.New("secret create result is unavailable")
	}
	result.Reused = true
	return result, nil
}

func (*provisionSecretSource) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("not implemented in provision fixture")
}
func (s *provisionSecretSource) Metadata(_ context.Context, handle secretcap.Handle) (secretcap.Metadata, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, result := range s.results {
		if result.Handle.Equal(handle) {
			return result.Metadata, nil
		}
	}
	return secretcap.Metadata{}, errors.New("secret is unavailable")
}
func (s *provisionSecretSource) Revoke(
	_ context.Context,
	handle secretcap.Handle,
	expected time.Time,
) (secretcap.Metadata, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for key, result := range s.results {
		if !result.Handle.Equal(handle) {
			continue
		}
		if !result.Metadata.Configured {
			return result.Metadata, nil
		}
		if !result.Metadata.UpdatedAt.Equal(expected) {
			return secretcap.Metadata{}, errors.New("secret version conflict")
		}
		s.revokeCount++
		result.Metadata.Configured = false
		result.Metadata.MaskedTail = ""
		result.Metadata.UpdatedAt = time.Now().UTC()
		s.results[key] = result
		return result.Metadata, nil
	}
	return secretcap.Metadata{}, errors.New("secret is unavailable")
}
func (*provisionSecretSource) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, errors.New("not implemented in provision fixture")
}

func (s *provisionSecretSource) createCalls() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.callCount
}
func (s *provisionSecretSource) producerCalls() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.producerCount
}
func (s *provisionSecretSource) revokeCalls() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.revokeCount
}
func (s *provisionSecretSource) plaintext() []byte {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return append([]byte(nil), s.plain...)
}

type provisionSecretAccessor struct{}

func (provisionSecretAccessor) Access(context.Context, secretcap.AccessRequest, secretcap.Consumer) error {
	return errors.New("not implemented in provision fixture")
}

func newProvisionSourceDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(provisionSourceDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL provision source tests", provisionSourceDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create provision admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate provision schema: %v", err)
	}
	schema := "linapro_resource_provision_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create provision schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendProvisionDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create scoped provision database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		if !strings.HasPrefix(schema, "linapro_resource_provision_test_") {
			t.Errorf("refusing to drop unexpected provision schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop provision schema: %v", dropErr)
		}
		_ = admin.Close(ctx)
	})
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve provision migration path")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve provision migrations: %v", err)
	}
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read provision migration: %v", readErr)
		}
		if _, executeErr := database.Exec(ctx, string(migration)); executeErr != nil {
			t.Fatalf("execute provision migration %s: %v", filepath.Base(path), executeErr)
		}
	}
	return ctx, database
}

func appendProvisionDSNParameter(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func principalReady(context.Context, string, string) (bool, error) {
	return true, nil
}

func TestProvisionTaskStorageFailureRetainsRetryCredentialPostgreSQL(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, db)
	secrets := newProvisionSecretSource(t)
	source, err := New(db, secrets, principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `CREATE FUNCTION reject_provision_task() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated task store unavailable'; END $$; CREATE TRIGGER reject_provision_task BEFORE INSERT ON plugin_idc_resource_task FOR EACH ROW EXECUTE FUNCTION reject_provision_task()`); err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	if task, err := source.CreateTask(ctx, request); err == nil || task != nil {
		t.Fatal("task storage failure hidden")
	}
	if secrets.createCalls() != 1 || secrets.revokeCalls() != 0 {
		t.Fatal("failed task creation revoked retry credential")
	}
	count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_task`)
	if err != nil || count.Int() != 0 {
		t.Fatal("failed task insert survived rollback")
	}
	if _, err = db.Exec(ctx, `DROP TRIGGER reject_provision_task ON plugin_idc_resource_task`); err != nil {
		t.Fatal(err)
	}
	created, err := source.CreateTask(ctx, request)
	if err != nil || created == nil {
		t.Fatalf("retry failed: %v", err)
	}
	if secrets.createCalls() != 1 || secrets.revokeCalls() != 0 {
		t.Fatal("retry regenerated the staged credential")
	}
	assertProvisionPersistence(t, ctx, db, secrets, created.ProviderTaskNo)
}
