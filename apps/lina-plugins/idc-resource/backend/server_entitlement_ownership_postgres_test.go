// This file verifies local server ownership evidence, customer isolation and
// idempotent entitlement delivery, including corrected connector versions.

package backend

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/customer360summary"
	"lina-plugin-idc-resource/backend/internal/service/serveroperationsource"
	"lina-plugin-idc-resource/backend/internal/service/serverownership"
)

func TestServerEntitlementOwnershipProjectionPostgreSQL(t *testing.T) {
	ctx, database := newServerOwnershipDatabase(t)
	seedServerOwnershipResource(t, ctx, database, "10000", "1")

	service, err := serverownership.New(database)
	if err != nil {
		t.Fatal(err)
	}
	contributor, err := customer360summary.NewLazy(func() gdb.DB { return database })
	if err != nil {
		t.Fatal(err)
	}
	counts, err := contributor.BatchCount(ctx, []string{"10000"})
	if err != nil || len(counts) != 1 || counts[0].Total != 0 {
		t.Fatalf("unbound resource was counted as a server entitlement: %+v err=%v", counts, err)
	}
	binding := serverOwnershipBinding("10000", "1")
	mismatchedRelease := binding
	mismatchedRelease.ReleaseDigest = strings.Repeat("c", 64)
	if err = service.Bind(ctx, mismatchedRelease); err == nil {
		t.Fatal("resource from another frozen Provider release was accepted")
	}
	if err = service.Bind(ctx, binding); err != nil {
		t.Fatalf("bind server ownership failed: %v", err)
	}
	if err = service.Bind(ctx, binding); err != nil {
		t.Fatalf("idempotent bind failed: %v", err)
	}
	if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ALTER COLUMN product_no DROP NOT NULL,
    ALTER COLUMN product_version_no DROP NOT NULL,
    ALTER COLUMN product_name_zh_cn DROP NOT NULL,
    ALTER COLUMN product_name_en_us DROP NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_server_entitlement_projection
SET product_no=NULL,product_version_no=NULL,product_name_zh_cn=NULL,product_name_en_us=NULL
WHERE entitlement_no=?`, binding.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if err = service.Bind(ctx, binding); err != nil {
		t.Fatalf("durable event replay did not complete migrated product references: %v", err)
	}
	productReference, err := database.GetOne(ctx, `
SELECT product_no,product_version_no,product_name_zh_cn,product_name_en_us
FROM plugin_idc_resource_server_entitlement_projection
WHERE entitlement_no=?`, binding.EntitlementNo)
	if err != nil || productReference["product_no"].String() != binding.ProductNo ||
		productReference["product_version_no"].String() != binding.ProductVersionNo ||
		productReference["product_name_zh_cn"].String() != binding.ProductNameZhCN ||
		productReference["product_name_en_us"].String() != binding.ProductNameEnUS {
		t.Fatalf("replayed product reference=%v err=%v", productReference, err)
	}

	changed := binding
	changed.CustomerNo = "10001"
	if err = service.Bind(ctx, changed); err == nil {
		t.Fatal("same entitlement was rebound to another customer")
	}

	counts, err = contributor.BatchCount(ctx, []string{"10000", "10001"})
	if err != nil || len(counts) != 2 {
		t.Fatalf("server counts=%+v err=%v", counts, err)
	}
	byCustomer := make(map[string]int64, len(counts))
	for _, count := range counts {
		byCustomer[count.CustomerNo] = count.Total
	}
	if byCustomer["10000"] != 1 || byCustomer["10001"] != 0 {
		t.Fatalf("server ownership isolation failed: %+v", byCustomer)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_mapping
SET status_code='released',released_at=CURRENT_TIMESTAMP
WHERE mapping_no=?`, binding.MappingNo); err != nil {
		t.Fatal(err)
	}
	counts, err = contributor.BatchCount(ctx, []string{"10000"})
	if err != nil || len(counts) != 1 || counts[0].Total != 0 {
		t.Fatalf("released server remained in customer count: %+v err=%v", counts, err)
	}
}

func TestServerEntitlementOwnershipTransferPostgreSQL(t *testing.T) {
	ctx, database := newServerOwnershipDatabase(t)
	seedServerOwnershipResource(t, ctx, database, "10000", "4")
	binding := serverOwnershipBinding("10000", "4")
	owners, err := serverownership.New(database)
	if err != nil {
		t.Fatal(err)
	}
	if err = owners.Bind(ctx, binding); err != nil {
		t.Fatal(err)
	}
	source, err := serveroperationsource.New(database)
	if err != nil {
		t.Fatal(err)
	}
	request := serveroperationresourcecap.OwnershipTransferRequest{
		TransferNo: "TRF_server_4", ServiceNo: binding.EntitlementNo,
		SourceCustomerNo: binding.CustomerNo, TargetCustomerNo: "10001",
	}
	rollback := errors.New("force surrounding transfer rollback")
	if err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if transferErr := source.TransferOwnership(txCtx, request); transferErr != nil {
			return transferErr
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatalf("resource ownership did not join the surrounding transaction: %v", err)
	}
	row, err := database.GetOne(ctx, `SELECT customer_no,version
FROM plugin_idc_resource_server_entitlement_projection WHERE entitlement_no=?`, binding.EntitlementNo)
	if err != nil || row["customer_no"].String() != binding.CustomerNo || row["version"].Int64() != 1 {
		t.Fatalf("rolled back resource projection=%v err=%v", row, err)
	}
	if err = source.TransferOwnership(ctx, request); err != nil {
		t.Fatalf("transfer resource ownership: %v", err)
	}
	row, err = database.GetOne(ctx, `SELECT customer_no,version
FROM plugin_idc_resource_server_entitlement_projection WHERE entitlement_no=?`, binding.EntitlementNo)
	if err != nil || row["customer_no"].String() != "10001" || row["version"].Int64() != 2 {
		t.Fatalf("transferred resource projection=%v err=%v", row, err)
	}
	if err = source.TransferOwnership(ctx, request); err == nil {
		t.Fatal("stale source customer was accepted twice")
	}
	forged := request
	forged.SourceCustomerNo, forged.TargetCustomerNo = "10000", "10002"
	if err = source.TransferOwnership(ctx, forged); err == nil {
		t.Fatal("cross-customer resource transfer was accepted")
	}
}

func TestEntitlementCreatedSinkUsesFrozenLocalEvidencePostgreSQL(t *testing.T) {
	ctx, database := newServerOwnershipDatabase(t)
	seedServerOwnershipResource(t, ctx, database, "10000", "3")
	binding := serverOwnershipBinding("10000", "3")
	configurationSnapshotNo := "configuration_server_3"
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provision_outcome_receipt(
 tenant_id,receipt_no,task_no,service_no,operation_no,customer_no,
 configuration_snapshot_no,status_code,request_digest,task_evidence_digest,
 core_evidence_digest)
VALUES(0,?,?,?,?,?,?,'succeeded',?,?,?)`,
		"receipt_server_3", binding.ProviderTaskNo, binding.EntitlementNo,
		binding.SourceIntentNo, binding.CustomerNo, configurationSnapshotNo,
		strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	sink, err := serverownership.NewEntitlementCreatedSink(database)
	if err != nil {
		t.Fatal(err)
	}
	event := entitlementcreatedcap.Event{
		Version: entitlementcreatedcap.ContractVersion,
		EventNo: "event_server_3", EntitlementNo: binding.EntitlementNo,
		IntentNo: binding.SourceIntentNo, CustomerNo: binding.CustomerNo,
		ProductNo: "product_server_3", ProductVersionNo: "version_server_3",
		ProductNameZhCN: "云服务器 3", ProductNameEnUS: "Cloud server 3",
		ConfigurationSnapshotNo: configurationSnapshotNo,
		InterfaceInstanceNo:     binding.InterfaceInstanceNo,
		PluginID:                binding.ProviderPluginID,
		PluginReleaseVersion:    binding.PluginReleaseVersion,
		ContractVersion:         binding.FulfillmentContractVersion,
		DeclarationDigest:       binding.DeclarationDigest,
		OwnerID:                 serverownership.EntitlementOwnerID,
	}
	forgedCustomer := event
	forgedCustomer.CustomerNo = "10001"
	if receipt, deliverErr := sink.Deliver(ctx, forgedCustomer); deliverErr == nil || receipt != nil {
		t.Fatal("forged customer ownership was accepted")
	}
	forgedPlugin := event
	forgedPlugin.PluginID = "other-server-provider"
	if receipt, deliverErr := sink.Deliver(ctx, forgedPlugin); deliverErr == nil || receipt != nil {
		t.Fatal("forged Provider ownership evidence was accepted")
	}
	receipt, err := sink.Deliver(ctx, event)
	if err != nil || receipt == nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) ||
		receipt.OwnerReference != binding.MappingNo {
		t.Fatalf("entitlement receipt=%+v err=%v", receipt, err)
	}
	replayed, err := sink.Deliver(ctx, event)
	if err != nil || replayed == nil || *replayed != *receipt {
		t.Fatalf("idempotent entitlement delivery=%+v err=%v", replayed, err)
	}
	row, err := database.GetOne(ctx, `SELECT count(*) AS total
FROM plugin_idc_resource_server_entitlement_projection
WHERE customer_no=? AND entitlement_no=?`, binding.CustomerNo, binding.EntitlementNo)
	if err != nil || row["total"].Int() != 1 {
		t.Fatalf("server entitlement projection count=%d err=%v", row["total"].Int(), err)
	}
}

// TestEntitlementCreatedSinkSeparatesPurchasedAndExecutedVersionsPostgreSQL
// keeps paid references immutable while verifying the corrected channel itself.
func TestEntitlementCreatedSinkSeparatesPurchasedAndExecutedVersionsPostgreSQL(t *testing.T) {
	ctx, database := newServerOwnershipDatabase(t)
	seedServerOwnershipResource(t, ctx, database, "10000", "5")
	seedServerOwnershipResource(t, ctx, database, "10000", "6")
	purchased := serverOwnershipBinding("10000", "5")
	executed := serverOwnershipBinding("10000", "6")
	const snapshot = "configuration_corrected_6"
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_channel_binding
 SET product_declaration_digest=? WHERE interface_instance_no=?`, strings.Repeat("d", 64), executed.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(struct {
		Purchased  string `json:"purchasedInterfaceInstanceNo"`
		Correction struct {
			Interface string `json:"interfaceInstanceNo"`
		} `json:"recoveryConfiguration"`
	}{Purchased: purchased.InterfaceInstanceNo, Correction: struct {
		Interface string `json:"interfaceInstanceNo"`
	}{Interface: executed.InterfaceInstanceNo}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task SET owner_metadata=?::jsonb WHERE task_no=?`, string(metadata), executed.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_provision_outcome_receipt(
 tenant_id,receipt_no,task_no,service_no,operation_no,customer_no,configuration_snapshot_no,
 status_code,request_digest,task_evidence_digest,core_evidence_digest)
 VALUES(0,?,?,?,?,?,?,'succeeded',?,?,?)`, "receipt_corrected_6", executed.ProviderTaskNo,
		purchased.EntitlementNo, purchased.SourceIntentNo, purchased.CustomerNo, snapshot,
		strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	sink, err := serverownership.NewEntitlementCreatedSink(database)
	if err != nil {
		t.Fatal(err)
	}
	event := entitlementcreatedcap.Event{
		Version: entitlementcreatedcap.ContractVersion, EventNo: "event_corrected_6",
		EntitlementNo: purchased.EntitlementNo, IntentNo: purchased.SourceIntentNo, CustomerNo: purchased.CustomerNo,
		ProductNo: purchased.ProductNo, ProductVersionNo: purchased.ProductVersionNo,
		ProductNameZhCN: purchased.ProductNameZhCN, ProductNameEnUS: purchased.ProductNameEnUS,
		ConfigurationSnapshotNo: snapshot, InterfaceInstanceNo: purchased.InterfaceInstanceNo,
		PluginID: purchased.ProviderPluginID, PluginReleaseVersion: purchased.PluginReleaseVersion,
		ContractVersion: purchased.FulfillmentContractVersion, DeclarationDigest: purchased.DeclarationDigest,
		OwnerID: serverownership.EntitlementOwnerID,
	}
	for _, name := range []string{"customer", "snapshot", "interface", "plugin", "version", "declaration"} {
		forged := event
		switch name {
		case "customer":
			forged.CustomerNo = "10001"
		case "snapshot":
			forged.ConfigurationSnapshotNo = "configuration_other"
		case "interface":
			forged.InterfaceInstanceNo = executed.InterfaceInstanceNo
		case "plugin":
			forged.PluginID = "other-server-provider"
		case "version":
			forged.PluginReleaseVersion = executed.PluginReleaseVersion
		case "declaration":
			forged.DeclarationDigest = strings.Repeat("d", 64)
		}
		if receipt, deliverErr := sink.Deliver(ctx, forged); deliverErr == nil || receipt != nil {
			t.Fatalf("forged %s was accepted", name)
		}
	}
	for _, mismatch := range []struct{ column, invalid, original string }{
		{"provider_id", "foreign-provider", "zjmfcloud"},
		{"provider_contract_version", "v2", "v1"},
		{"provider_release_version", "v9.9.9", executed.PluginReleaseVersion},
		{"provider_release_digest", strings.Repeat("e", 64), executed.ReleaseDigest},
	} {
		// Only known fixture column identifiers are interpolated; values stay bound.
		query := "UPDATE plugin_idc_resource_channel SET " + mismatch.column + "=? WHERE channel_no=?"
		if _, err = database.Exec(ctx, query, mismatch.invalid, "channel_server_6"); err != nil {
			t.Fatal(err)
		}
		if receipt, deliverErr := sink.Deliver(ctx, event); deliverErr == nil || receipt != nil {
			t.Fatalf("mismatched %s was accepted", mismatch.column)
		}
		if _, err = database.Exec(ctx, query, mismatch.original, "channel_server_6"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task
 SET owner_metadata=jsonb_set(owner_metadata,'{recoveryConfiguration,interfaceInstanceNo}',to_jsonb(?::text)) WHERE task_no=?`, purchased.InterfaceInstanceNo, executed.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	if receipt, deliverErr := sink.Deliver(ctx, event); deliverErr == nil || receipt != nil {
		t.Fatal("execution outside the frozen correction was accepted")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task SET owner_metadata=?::jsonb WHERE task_no=?`, string(metadata), executed.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	receipt, err := sink.Deliver(ctx, event)
	if err != nil || receipt == nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) || receipt.OwnerReference != executed.MappingNo {
		t.Fatalf("corrected receipt=%+v err=%v", receipt, err)
	}
	replayed, err := sink.Deliver(ctx, event)
	if err != nil || replayed == nil || *replayed != *receipt {
		t.Fatalf("corrected replay=%+v err=%v", replayed, err)
	}
	row, err := database.GetOne(ctx, `SELECT customer_no,product_no,product_version_no,source_intent_no,
 interface_instance_no,plugin_release_version,declaration_digest,provider_task_no
 FROM plugin_idc_resource_server_entitlement_projection WHERE entitlement_no=?`, purchased.EntitlementNo)
	if err != nil {
		t.Fatal(err)
	}
	if row["customer_no"].String() != purchased.CustomerNo || row["product_no"].String() != purchased.ProductNo ||
		row["product_version_no"].String() != purchased.ProductVersionNo || row["source_intent_no"].String() != purchased.SourceIntentNo ||
		row["interface_instance_no"].String() != executed.InterfaceInstanceNo || row["plugin_release_version"].String() != executed.PluginReleaseVersion ||
		row["declaration_digest"].String() != strings.Repeat("d", 64) || row["provider_task_no"].String() != executed.ProviderTaskNo {
		t.Fatalf("corrected ownership=%v", row)
	}
}

func TestServerEntitlementOwnershipRequiresSucceededResourceEvidencePostgreSQL(t *testing.T) {
	ctx, database := newServerOwnershipDatabase(t)
	seedServerOwnershipResource(t, ctx, database, "10000", "2")
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_task
SET status_code='failed',finished_at=CURRENT_TIMESTAMP,failure_code='known_failure',failure_summary='known failure'
WHERE task_no=?`, "task_server_2"); err != nil {
		t.Fatal(err)
	}
	service, err := serverownership.New(database)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Bind(ctx, serverOwnershipBinding("10000", "2")); err == nil {
		t.Fatal("failed Provider task was accepted as server ownership evidence")
	}
	row, err := database.GetOne(ctx, `SELECT count(*) AS total
FROM plugin_idc_resource_server_entitlement_projection`)
	if err != nil || row["total"].Int() != 0 {
		total := row["total"].Int()
		t.Fatalf("invalid projection rows=%d err=%v", total, err)
	}
}

func newServerOwnershipDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(testEnvironment(t, resourceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL server ownership tests", resourceIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newResourceIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteResourcePostgreSQLIdentifier(schema)); err != nil {
		_ = admin.Close(ctx)
		t.Fatal(err)
	}
	database, err := gdb.New(gdb.ConfigNode{
		Link: appendResourceDSNParameter(dsn, "search_path="+schema),
	})
	if err != nil {
		_ = admin.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		if strings.HasPrefix(schema, "linapro_resource_test_") {
			_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteResourcePostgreSQLIdentifier(schema)+" CASCADE")
		}
		_ = admin.Close(ctx)
	})
	executeResourceMigrations(t, ctx, database)
	return ctx, database
}

func testEnvironment(t *testing.T, name string) string {
	t.Helper()
	return os.Getenv(name)
}

func seedServerOwnershipResource(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	suffix string,
) {
	t.Helper()
	binding := serverOwnershipBinding(customerNo, suffix)
	statements := []struct {
		query string
		args  []any
	}{
		{query: `INSERT INTO plugin_idc_resource_channel
 (channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,
  provider_release_version,provider_release_digest,provider_declaration_digest,
  display_name,endpoint,status_code)
 VALUES (?,?,?,'dynamic','v1',?,?,?,?,?,'enabled')`, args: []any{
			"channel_server_" + suffix, "zjmfcloud", binding.ProviderPluginID,
			binding.PluginReleaseVersion, binding.ReleaseDigest, binding.DeclarationDigest,
			"Server channel", "https://supplier.example",
		}},
		{query: `INSERT INTO plugin_idc_resource_pool
 (pool_no,channel_no,display_name,resource_kind,status_code)
		 VALUES (?,?,?,'virtual_server','enabled')`, args: []any{"pool_server_" + suffix, "channel_server_" + suffix, "Server pool"}},
		{query: `INSERT INTO plugin_idc_resource_interface_channel_binding
 (interface_instance_no,channel_no,binding_version,display_name,
  resource_plugin_id,resource_plugin_release_version,resource_contract_version,
  resource_authorized_methods,resource_release_digest,resource_declaration_digest,
  product_plugin_id,product_plugin_release_version,product_contract_version,
  product_authorized_methods,product_release_digest,product_declaration_digest,new_business_state)
 VALUES (?,?,1,'Server channel',?,?, 'v1',ARRAY['configuration.candidates'],?,?,
         ?,?,'v1',ARRAY['provision','query'],?,?,'accepting')`, args: []any{
			binding.InterfaceInstanceNo, "channel_server_" + suffix,
			binding.ProviderPluginID, binding.PluginReleaseVersion, binding.ReleaseDigest, binding.DeclarationDigest,
			binding.ProviderPluginID, binding.PluginReleaseVersion, binding.ReleaseDigest, binding.DeclarationDigest,
		}},
		{query: `INSERT INTO plugin_idc_resource_task
 (task_no,request_id_digest,capability_code,local_resource_no,channel_no,pool_no,
  status_code,attempt_count,max_attempts,finished_at)
 VALUES (?,?,?,?,?,?,'succeeded',1,8,CURRENT_TIMESTAMP)`, args: []any{binding.ProviderTaskNo, strings.Repeat(suffix, 64), resourcecap.CapabilityProvision, binding.LocalResourceNo, "channel_server_" + suffix, "pool_server_" + suffix}},
		{query: `INSERT INTO plugin_idc_resource_mapping
 (mapping_no,local_resource_no,channel_no,pool_no,external_resource_ref,status_code)
 VALUES (?,?,?,?,?,'active')`, args: []any{binding.MappingNo, binding.LocalResourceNo, "channel_server_" + suffix, "pool_server_" + suffix, "upstream-server-" + suffix}},
	}
	for _, statement := range statements {
		if _, err := database.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed server ownership resource failed: %v", err)
		}
	}
}

func serverOwnershipBinding(customerNo string, suffix string) resourcecap.ServerEntitlementBinding {
	digit := strings.Repeat(suffix, 32)
	return resourcecap.ServerEntitlementBinding{
		Version:                    resourcecap.ServerEntitlementProjectionVersion,
		EntitlementNo:              "ent_" + digit,
		SourceIntentNo:             "intent_server_" + suffix,
		CustomerNo:                 customerNo,
		ProductNo:                  "product_server_" + suffix,
		ProductVersionNo:           "version_server_" + suffix,
		ProductNameZhCN:            "云服务器 " + suffix,
		ProductNameEnUS:            "Cloud server " + suffix,
		LocalResourceNo:            "SRV_" + digit,
		MappingNo:                  "mapping_server_" + suffix,
		ProviderTaskNo:             "task_server_" + suffix,
		InterfaceInstanceNo:        "pfi_" + digit,
		ProviderPluginID:           "idc-zjmfcloud",
		PluginReleaseVersion:       "v1.0." + suffix,
		FulfillmentContractVersion: "v1",
		ServerContractVersion:      resourcecap.ServerFulfillmentContractVersion,
		ReleaseDigest:              strings.Repeat("a", 64),
		DeclarationDigest:          strings.Repeat("b", 64),
	}
}
