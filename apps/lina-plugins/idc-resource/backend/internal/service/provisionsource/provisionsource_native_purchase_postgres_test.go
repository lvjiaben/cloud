// This file proves generic native procurement persists one owner correlation
// without server credentials or routing, and preserves replay/outcome fencing.
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
	"time"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"lina-plugin-idc-resource/backend/internal/service/serverownership"
)

type nativePurchaseTestBuilder struct {
	provisionManagedBuilder
	calls int
}

func (b *nativePurchaseTestBuilder) BuildNative(_ context.Context, channel, revision string, plan resourcecap.NativePurchasePlan) (json.RawMessage, error) {
	b.calls++
	if channel != "channel-provision-1" || revision != "rci_22222222222222222222222222222222" || plan.Validate() != nil || plan.Purchase.PublicSlug != "41" {
		return nil, errors.New("native mapper evidence mismatch")
	}
	return json.Marshal(map[string]string{"pid": "41", "billingcycle": "monthly", "workflow": "serialized_cart_v1"})
}

func TestNativePurchaseTaskPostgreSQLNoServerAssumptions(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, db)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("native migration path unavailable")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "042-native-purchase-mapping.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = db.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`DELETE FROM plugin_idc_resource_pool`,
		`DELETE FROM plugin_idc_resource_principal_mapping`,
		`UPDATE plugin_idc_resource_capability_snapshot_item SET constraints='[{"name":"workflow","required":true,"allowedValues":["serialized_cart_v1"]}]' WHERE capability_code='server.provision'`,
		`INSERT INTO plugin_idc_resource_interface_configuration_revision(revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by) VALUES('rci_22222222222222222222222222222222','pfi_11111111111111111111111111111111','channel-provision-1','https://provider.invalid','{"__host":{"policy":{"mode":"per_request_form","mappings":[{"role":"username","slot":"username"},{"role":"password","slot":"password"}],"callback":true}}}','{}',101)`,
	} {
		if _, err := db.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	secrets := newProvisionSecretSource(t)
	builder := &nativePurchaseTestBuilder{}
	principalCalls := 0
	svc, err := New(db, secrets, func(context.Context, string, string) (bool, error) {
		principalCalls++
		return false, errors.New("native purchase must not provision a customer account")
	}, nil, nil, builder)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetPublicOriginSource(func() string { return "https://shop.example" })
	request := frozenProvisionSourceRequest(t)
	request.InterfaceConfigurationRevision = "rci_22222222222222222222222222222222"
	request.ProductConfiguration = json.RawMessage(`{"upstream_product_id":"41","upstream_configuration_revision":"revision-native"}`)
	request.CustomerConfiguration = json.RawMessage(`{}`)
	request.NativePurchaseProtocol = productfulfillmentcap.SaleImportProtocolNativeFinanceV1
	request.NativePurchase = &channelcommercecap.Purchase{PublicSlug: "41", ConfigurationRevision: "revision-native", CurrencyCode: "CNY", BillingTermToken: "monthly", PurchaseCycles: 1, Quantity: 1}
	request.RequestDigest, err = provisionresourcecap.TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateTask(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := svc.CreateTask(ctx, request)
	if err != nil || replay.ProviderTaskNo != created.ProviderTaskNo || !replay.Reused || builder.calls != 1 || principalCalls != 0 {
		t.Fatalf("native replay: %+v calls=%d principal=%d err=%v", replay, builder.calls, principalCalls, err)
	}
	var facts struct {
		PoolNo, LocalResourceNo, BindingNames string
		Correlations, Credentials             int
	}
	if err = db.GetScan(ctx, &facts, `SELECT COALESCE(task.pool_no,'') pool_no,task.local_resource_no,
 (SELECT string_agg(binding_name,',') FROM plugin_idc_resource_task_secret_binding WHERE task_no=task.task_no) binding_names,
 (SELECT count(*)::INT FROM plugin_idc_resource_provider_correlation WHERE task_no=task.task_no) correlations,
 (SELECT count(*)::INT FROM plugin_idc_resource_provision_credential WHERE task_no=task.task_no) credentials
 FROM plugin_idc_resource_task task WHERE task.task_no=?`, created.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	if facts.PoolNo != "" || !strings.HasPrefix(facts.LocalResourceNo, "NAT_") || facts.BindingNames != "downstream_token" || facts.Correlations != 1 || facts.Credentials != 0 || len(secrets.plaintext()) != 32 {
		t.Fatalf("native purchase used server state: %+v", facts)
	}
	changed := request
	purchase := *request.NativePurchase
	purchase.BillingTermToken = "annually"
	changed.NativePurchase = &purchase
	changed.RequestDigest, err = provisionresourcecap.TaskRequestDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CreateTask(ctx, changed); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
		t.Fatalf("changed paid purchase accepted: %v", err)
	}
	store, err := resourcetask.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := store.Claim(ctx, resourcetask.ClaimInput{WorkerID: "native-worker", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(claims) != 1 {
		t.Fatalf("native task claim: %+v %v", claims, err)
	}
	claim := claims[0]
	if claim.ProviderCorrelation == nil || len(claim.SecretBindings) != 1 || claim.SecretBindings[0].Name != "downstream_token" {
		t.Fatal("native execution lost owner correlation")
	}
	if err = store.Transition(ctx, resourcetask.TransitionInput{Lease: resourcetask.LeaseRef{TaskNo: claim.TaskNo, Token: claim.LeaseToken, AttemptNo: claim.AttemptNo}, To: resourcetask.StatusSucceeded, Evidence: resourcetask.TransitionEvidence{ObservationProven: true}, ResourceRef: "upstream-41"}); err != nil {
		t.Fatal(err)
	}
	completed, err := svc.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: created.ProviderTaskNo, Request: request})
	if err != nil {
		t.Fatal(err)
	}
	report := provisionresourcecap.OutcomeReport{ProviderTaskNo: created.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusSucceeded, TaskEvidenceDigest: completed.EvidenceDigest, CoreEvidenceDigest: strings.Repeat("e", 64)}
	for range 2 {
		if err = svc.ReportOutcome(ctx, report); err != nil {
			t.Fatal(err)
		}
	}
	count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_provision_outcome_receipt WHERE task_no=?`, created.ProviderTaskNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("native receipt replay: %v %v", count, err)
	}

	sink, err := serverownership.NewEntitlementCreatedSink(db)
	if err != nil {
		t.Fatal(err)
	}
	event := entitlementcreatedcap.Event{Version: entitlementcreatedcap.ContractVersion, EventNo: "event-native", EntitlementNo: "ent_22222222222222222222222222222222", IntentNo: request.OperationNo, CustomerNo: request.CustomerNo, ProductNo: request.Fulfillment.ProductNo, ProductVersionNo: request.Fulfillment.ProductVersionNo, ProductNameZhCN: "原生通用商品", ProductNameEnUS: "Native generic product", ConfigurationSnapshotNo: request.ConfigurationSnapshotNo, InterfaceInstanceNo: request.InterfaceInstanceNo, PluginID: "idc-zjmfcloud", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", DeclarationDigest: strings.Repeat("b", 64), OwnerID: serverownership.EntitlementOwnerID}
	receipt, err := sink.Deliver(ctx, event)
	if err != nil || receipt == nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) {
		t.Fatalf("native event receipt: %+v %v", receipt, err)
	}
	serverRows, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_server_entitlement_projection WHERE entitlement_no=?`, event.EntitlementNo)
	if err != nil || serverRows.Int() != 0 {
		t.Fatal("native generic purchase became a fake server")
	}
}
