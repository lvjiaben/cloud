// These tests exercise corrected native mappings using an isolated database and
// a simulated provider. No upstream requests, accounts or resources are created.
package provisionsource

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverownership"
)

type correctionCandidates struct {
	calls    int
	revision string
	fail     bool
}

func (c *correctionCandidates) ResolveCandidates(_ context.Context, input resourceinterfaceprojectioncap.CandidateRequest) (*resourceinterfaceprojectioncap.CandidateResult, error) {
	c.calls++
	if c.fail {
		return nil, errors.New("fixture supplier unavailable")
	}
	expected := c.revision
	if expected == "" {
		expected = "rci_11111111111111111111111111111111"
	}
	if input.ExpectedConfigurationRevision != expected {
		return nil, errors.New("unexpected revision")
	}
	return &resourceinterfaceprojectioncap.CandidateResult{Fields: []resourceinterfaceprojectioncap.CandidateField{
		{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "1", LabelZhCN: "修正区域", LabelEnUS: "Corrected region"}}},
		{FieldKey: "line", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "5", LabelZhCN: "修正节点", LabelEnUS: "Corrected node"}}},
		{FieldKey: "operating_system", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "98", LabelZhCN: "可用镜像", LabelEnUS: "Available image"}}},
	}}, nil
}

type correctionBuilder struct {
	plans []resourcecap.ServerProvisionPlan
}

func TestRecoveryReadFailureKeepsReplacementChoicePostgreSQL(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, db)
	candidates := &correctionCandidates{fail: true}
	builder := &correctionBuilder{}
	source, err := New(db, newProvisionSecretSource(t), principalReady, candidates, nil, builder)
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	editor, err := source.InspectRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request})
	if err != nil || editor == nil {
		t.Fatalf("read outage hid recovery editor: %v", err)
	}
	if len(editor.Interfaces) != 1 || len(editor.Specifications) == 0 || editor.Original.InterfaceLabel == "" || editor.Selection.InterfaceInstanceNo != request.InterfaceInstanceNo {
		t.Fatal("read outage discarded local terms or replacement channels")
	}
	if editor.Selection.Region != "" || editor.Selection.Line != "" || editor.Selection.OperatingSystem != "" {
		t.Fatal("unverified option was preserved as selectable")
	}
	if _, err := source.ValidateRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: &editor.Selection}); !errors.Is(err, provisionresourcecap.ErrParametersUnavailable) {
		t.Fatalf("read outage admitted a retry: %v", err)
	}
	if len(builder.plans) != 0 {
		t.Fatal("inspection performed a write")
	}
	candidates.fail = false
	editor, err = source.InspectRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request})
	if err != nil || len(editor.Regions) != 1 || len(editor.OperatingSystems) != 1 {
		t.Fatal("editor did not recover after supplier read recovered")
	}
}

func (b *correctionBuilder) Build(_ context.Context, _ string, _ string, plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	b.plans = append(b.plans, plan)
	return json.Marshal(map[string]any{"num": 1, "cpu": 2, "memory": 4096, "os": "98", "client": plan.ExternalPrincipalRef, "hostname": plan.Hostname})
}

func TestOperationalCorrectionKeepsPurchasedTermsPostgreSQL(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, db)
	candidates, builder := &correctionCandidates{}, &correctionBuilder{}
	secrets := newProvisionSecretSource(t)
	source, err := New(db, secrets, principalReady, candidates, nil, builder)
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	originalProduct, originalCustomer := string(request.ProductConfiguration), string(request.CustomerConfiguration)
	original, err := source.CreateTask(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='failed',failure_code='UPSTREAM_REQUEST_REJECTED',result_unknown=FALSE,finished_at=CURRENT_TIMESTAMP,version=version+1 WHERE task_no=?`, original.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	previous := provisionresourcecap.TaskReadRequest{ProviderTaskNo: original.ProviderTaskNo, Request: request}
	failed, err := source.ReadTask(ctx, previous)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{ProviderTaskNo: original.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusFailed, TaskEvidenceDigest: failed.EvidenceDigest, CoreEvidenceDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	selection := &provisionresourcecap.RecoveryConfiguration{InterfaceInstanceNo: request.InterfaceInstanceNo, Region: "1", Line: "5", OperatingSystem: "98", PasswordLength: 12, PasswordCharacters: "uppercase,digits"}
	editor, err := source.InspectRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: selection})
	if err != nil {
		t.Fatal(err)
	}
	correction, err := source.ValidateRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: &editor.Selection})
	if err != nil {
		t.Fatal(err)
	}
	previous.RecoveryConfiguration = correction
	next, err := source.RecoverRejected(ctx, previous)
	if err != nil || next.ProviderTaskNo == original.ProviderTaskNo {
		t.Fatalf("corrected successor=%+v %v", next, err)
	}
	if len(builder.plans) != 2 {
		t.Fatalf("unexpected writes/builds: %d", len(builder.plans))
	}
	password := secrets.plaintext()
	if len(password) != 12 || strings.Trim(string(password), "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" || !strings.ContainsAny(string(password), "0123456789") || !strings.ContainsAny(string(password), "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatal("recovery did not use corrected password generation rules")
	}
	row, readErr := source.loadTaskByNumber(ctx, next.ProviderTaskNo)
	if readErr != nil || row == nil || strings.Contains(row.OwnerMetadata, string(password)) {
		t.Fatal("recovery leaked a password")
	}
	corrected := builder.plans[1]
	if string(corrected.Selector) != directPoolSelector {
		t.Fatal("old merchant mapping still overrides corrected native values")
	}
	var product map[string]any
	var customer map[string]any
	if json.Unmarshal(corrected.ProductConfiguration, &product) != nil || json.Unmarshal(corrected.CustomerConfiguration, &customer) != nil {
		t.Fatal("invalid corrected plan")
	}
	if product["cpu"] != float64(2) || product["memory"] != float64(4096) || product["system_disk"] != float64(40) || product["bandwidth"] != float64(100) || product["region"] != "1" || product["line"] != "5" || customer["operating_system"] != "98" {
		t.Fatalf("purchased terms changed or correction lost: %v %v", product, customer)
	}
	if string(request.ProductConfiguration) != originalProduct || string(request.CustomerConfiguration) != originalCustomer {
		t.Fatal("original paid request mutated")
	}
	if _, err = source.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: next.ProviderTaskNo, Request: request}); err != nil {
		t.Fatalf("corrected task lost original paid identity: %v", err)
	}
	replay, err := source.RecoverRejected(ctx, previous)
	if err != nil || replay.ProviderTaskNo != next.ProviderTaskNo || len(builder.plans) != 2 {
		t.Fatalf("duplicate successor: %+v %v", replay, err)
	}
	changed := *correction
	changed.PasswordLength = 13
	previous.RecoveryConfiguration = &changed
	if _, err = source.RecoverRejected(ctx, previous); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
		t.Fatalf("idempotent task silently rebound: %v", err)
	}
	invalid := *correction
	invalid.Line = "not-an-option"
	if _, err = source.ValidateRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: &invalid}); !errors.Is(err, provisionresourcecap.ErrParametersUnavailable) {
		t.Fatalf("invented node accepted: %v", err)
	}
	stale := *correction
	stale.InterfaceConfigurationRevision = "rci_ffffffffffffffffffffffffffffffff"
	if _, err = source.ValidateRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: &stale}); !errors.Is(err, provisionresourcecap.ErrParametersUnavailable) {
		t.Fatalf("stale connection accepted: %v", err)
	}
}

// A disabled old channel does not prevent an explicitly selected compatible
// replacement. Its new connection and upstream principal are pinned separately.
func TestCorrectionCanSelectAnotherUpstreamPostgreSQL(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resource test path is unavailable")
	}
	indexSQL, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/039-provisioning-coordination-indexes.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.Exec(ctx, string(indexSQL)); err != nil {
			t.Fatalf("owner receipt index is not safely repeatable: %v", err)
		}
	}
	definition, err := db.GetValue(ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname=current_schema() AND indexname='idx_plugin_idc_resource_provision_outcome_identity'`)
	if err != nil || !strings.Contains(definition.String(), "(tenant_id, operation_no, customer_no, configuration_snapshot_no)") || !strings.Contains(definition.String(), "succeeded") {
		t.Fatalf("owner receipt lookup has no matching identity index: %v %v", definition, err)
	}
	latestIndex, err := db.GetValue(ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname=current_schema() AND indexname='idx_plugin_idc_resource_cap_attempt_latest'`)
	if err != nil || !strings.Contains(latestIndex.String(), "(tenant_id, channel_no, observed_at DESC, id DESC)") {
		t.Fatalf("latest discovery has no matching attempt index: %v %v", latestIndex, err)
	}
	seedProvisionRoute(t, ctx, db)
	builder, candidates := &correctionBuilder{}, &correctionCandidates{revision: "rci_22222222222222222222222222222222"}
	if _, err := db.Exec(ctx, `CREATE TABLE fixture_principal_preparation(customer_no TEXT,pool_no TEXT,PRIMARY KEY(customer_no,pool_no))`); err != nil {
		t.Fatal(err)
	}
	preparationPending := false
	ensurePrincipal := func(ctx context.Context, customerNo, poolNo string) (bool, error) {
		if !preparationPending {
			return true, nil
		}
		_, err := db.Exec(ctx, `INSERT INTO fixture_principal_preparation VALUES(?,?) ON CONFLICT DO NOTHING`, customerNo, poolNo)
		return false, err
	}
	source, err := New(db, newProvisionSecretSource(t), ensurePrincipal, candidates, nil, builder)
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	previous, err := source.CreateTask(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='failed',failure_code='UPSTREAM_REQUEST_REJECTED',result_unknown=FALSE,finished_at=CURRENT_TIMESTAMP,version=version+1 WHERE task_no=?`, previous.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	oldRead := provisionresourcecap.TaskReadRequest{ProviderTaskNo: previous.ProviderTaskNo, Request: request}
	failed, err := source.ReadTask(ctx, oldRead)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{ProviderTaskNo: previous.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusFailed, TaskEvidenceDigest: failed.EvidenceDigest, CoreEvidenceDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel(channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,display_name,endpoint,status_code)
 SELECT 'channel-provision-2',provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,'Replacement','https://replacement.invalid','enabled' FROM plugin_idc_resource_channel WHERE channel_no='channel-provision-1';
INSERT INTO plugin_idc_resource_interface_channel_binding(interface_instance_no,channel_no,binding_version,current_configuration_revision,product_plugin_id,product_plugin_release_version,product_contract_version,product_authorized_methods,product_release_digest,product_declaration_digest)
 SELECT 'pfi_22222222222222222222222222222222','channel-provision-2',1,'rci_22222222222222222222222222222222',product_plugin_id,product_plugin_release_version,product_contract_version,product_authorized_methods,product_release_digest,product_declaration_digest FROM plugin_idc_resource_interface_channel_binding LIMIT 1;
INSERT INTO plugin_idc_resource_interface_configuration_revision(revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by)
 VALUES('rci_22222222222222222222222222222222','pfi_22222222222222222222222222222222','channel-provision-2','https://replacement.invalid','{}','{}',101);
INSERT INTO plugin_idc_resource_principal_mapping(mapping_no,customer_no,channel_no,external_principal_ref,status_code,evidence_digest,observed_at)
 VALUES('principal-provision-2','customer-provision-1','channel-provision-2','99','active',repeat('a',64),CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_resource_capability_snapshot(snapshot_no,channel_no,adapter_version,status_code,item_count,observed_at,completed_at)
 SELECT 'capability-provision-2','channel-provision-2',adapter_version,status_code,item_count,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP FROM plugin_idc_resource_capability_snapshot LIMIT 1;
INSERT INTO plugin_idc_resource_capability_snapshot_item(snapshot_no,capability_code,support_code,constraints,unsupported_reason)
 SELECT 'capability-provision-2',capability_code,support_code,constraints,unsupported_reason FROM plugin_idc_resource_capability_snapshot_item WHERE snapshot_no='capability-provision-1';
UPDATE plugin_idc_resource_channel SET status_code='disabled',version=version+1 WHERE channel_no='channel-provision-1';`)
	if err != nil {
		t.Fatal(err)
	}
	selection := &provisionresourcecap.RecoveryConfiguration{InterfaceInstanceNo: "pfi_22222222222222222222222222222222", Region: "1", Line: "5", OperatingSystem: "98"}
	initial, err := source.InspectRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request})
	if err != nil || len(initial.Interfaces) != 1 || initial.Selection.InterfaceInstanceNo != "" || len(initial.Specifications) == 0 {
		t.Fatalf("disabled original prevents replacement selection: %+v %v", initial, err)
	}
	editor, err := source.InspectRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: selection})
	if err != nil {
		t.Fatal(err)
	}
	correction, err := source.ValidateRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: &editor.Selection})
	if err != nil {
		t.Fatal(err)
	}
	oldRead.RecoveryConfiguration = correction
	preparationPending = true
	if _, err := source.RecoverRejected(ctx, oldRead); !errors.Is(err, provisionresourcecap.ErrSourceUnavailable) {
		t.Fatalf("unprepared upstream account was not deferred: %v", err)
	}
	prepared, err := db.GetValue(ctx, `SELECT count(*) FROM fixture_principal_preparation`)
	if err != nil || prepared.Int() != 1 {
		t.Fatalf("upstream account preparation was rolled back with deferred recovery: count=%v err=%v", prepared, err)
	}
	taskCount, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_task`)
	if err != nil || taskCount.Int() != 1 {
		t.Fatalf("pending account created a provider successor: count=%v err=%v", taskCount, err)
	}
	preparationPending = false
	next, err := source.RecoverRejected(ctx, oldRead)
	if err != nil || next.ChannelNo != "channel-provision-2" {
		t.Fatalf("replacement upstream not used: %+v %v", next, err)
	}
	if len(builder.plans) != 2 || builder.plans[1].ExternalPrincipalRef != "99" {
		t.Fatalf("wrong upstream principal: %+v", builder.plans)
	}
	row, err := source.loadTaskByNumber(ctx, next.ProviderTaskNo)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := decodeTaskOwnerFacts(row.OwnerMetadata)
	if err != nil || facts.InterfaceConfigurationRevision != "rci_22222222222222222222222222222222" || facts.RecoveryConfiguration == nil || facts.RecoveryConfiguration.InterfaceLabel != "Replacement" {
		t.Fatalf("replacement revision/history missing: %+v %v", facts, err)
	}
	// The actual succeeded task belongs to the replacement channel, while the
	// paid event keeps its original interface and customer identity.
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='succeeded',finished_at=CURRENT_TIMESTAMP,version=version+1 WHERE task_no=?`, next.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_mapping(mapping_no,local_resource_no,channel_no,pool_no,external_resource_ref,status_code) VALUES('mapping-corrected',?,'channel-provision-2',?,'cloud-corrected','active')`, row.LocalResourceNo, row.PoolNo); err != nil {
		t.Fatal(err)
	}
	succeeded, err := source.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: next.ProviderTaskNo, Request: request})
	if err != nil {
		t.Fatal(err)
	}
	if err = source.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{ProviderTaskNo: next.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusSucceeded, TaskEvidenceDigest: succeeded.EvidenceDigest, CoreEvidenceDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	sink, err := serverownership.NewEntitlementCreatedSink(db)
	if err != nil {
		t.Fatal(err)
	}
	event := entitlementcreatedcap.Event{Version: entitlementcreatedcap.ContractVersion, EventNo: "event-corrected-server", EntitlementNo: "ent_22222222222222222222222222222222", IntentNo: request.OperationNo, CustomerNo: request.CustomerNo, ProductNo: request.Fulfillment.ProductNo, ProductVersionNo: request.Fulfillment.ProductVersionNo, ProductNameZhCN: "修正上游服务器", ProductNameEnUS: "Corrected cloud server", ConfigurationSnapshotNo: request.ConfigurationSnapshotNo, InterfaceInstanceNo: request.InterfaceInstanceNo, PluginID: "idc-zjmfcloud", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", DeclarationDigest: strings.Repeat("b", 64), OwnerID: serverownership.EntitlementOwnerID}
	wrong := event
	wrong.CustomerNo = "another-customer"
	if _, err = sink.Deliver(ctx, wrong); err == nil {
		t.Fatal("corrected server became another customer's entitlement")
	}
	wrong = event
	wrong.InterfaceInstanceNo = correction.InterfaceInstanceNo
	if _, err = sink.Deliver(ctx, wrong); err == nil {
		t.Fatal("operational correction rewrote the paid interface identity")
	}
	receipt, err := sink.Deliver(ctx, event)
	if err != nil || receipt == nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) {
		t.Fatalf("replacement server cannot bind to its original paid entitlement: receipt=%+v err=%v", receipt, err)
	}
	replay, err := sink.Deliver(ctx, event)
	if err != nil || replay == nil || *replay != *receipt {
		t.Fatalf("replacement ownership replay changed its receipt: %+v %v", replay, err)
	}
	projected, err := db.GetOne(ctx, `SELECT customer_no,interface_instance_no,provider_task_no FROM plugin_idc_resource_server_entitlement_projection WHERE entitlement_no=?`, event.EntitlementNo)
	if err != nil || projected["customer_no"].String() != request.CustomerNo || projected["interface_instance_no"].String() != correction.InterfaceInstanceNo || projected["provider_task_no"].String() != next.ProviderTaskNo {
		t.Fatalf("replacement ownership projection lost executed routing: %v %v", projected, err)
	}
}

func (c *correctionCandidates) CatalogAvailability(context.Context, []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}
