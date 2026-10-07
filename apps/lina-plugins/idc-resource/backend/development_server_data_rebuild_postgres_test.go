// This file verifies that development server projections can be rebuilt from frozen entitlement events without replaying Provider writes.

package backend

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverownership"
)

func TestDevelopmentServerDataProjectionRebuildAndReconciliationPostgreSQL(t *testing.T) {
	ctx, database := newServerOwnershipDatabase(t)
	seedServerOwnershipResource(t, ctx, database, "10000", "5")
	binding := serverOwnershipBinding("10000", "5")
	configurationSnapshotNo := "configuration_server_5"
	seedDevelopmentServerProvisionReceipt(t, ctx, database, binding, configurationSnapshotNo)

	sink, err := serverownership.NewEntitlementCreatedSink(database)
	if err != nil {
		t.Fatal(err)
	}
	event := entitlementcreatedcap.Event{
		Version: entitlementcreatedcap.ContractVersion,
		EventNo: "event_server_5", EntitlementNo: binding.EntitlementNo,
		IntentNo: binding.SourceIntentNo, CustomerNo: binding.CustomerNo,
		ProductNo: "product_server_5", ProductVersionNo: "version_server_5",
		ProductNameZhCN: "开发库云服务器", ProductNameEnUS: "Development cloud server",
		ConfigurationSnapshotNo: configurationSnapshotNo,
		InterfaceInstanceNo:     binding.InterfaceInstanceNo,
		PluginID:                binding.ProviderPluginID,
		PluginReleaseVersion:    binding.PluginReleaseVersion,
		ContractVersion:         binding.FulfillmentContractVersion,
		DeclarationDigest:       binding.DeclarationDigest,
		OwnerID:                 serverownership.EntitlementOwnerID,
	}

	receipt, err := sink.Deliver(ctx, event)
	if err != nil || receipt == nil || !entitlementcreatedcap.ValidateReceipt(event, *receipt) {
		t.Fatalf("initial development projection delivery=%+v err=%v", receipt, err)
	}
	replayed, err := sink.Deliver(ctx, event)
	if err != nil || replayed == nil || *replayed != *receipt {
		t.Fatalf("idempotent development projection replay=%+v err=%v", replayed, err)
	}

	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_resource_server_entitlement_projection
WHERE "entitlement_no"=?`, event.EntitlementNo); err != nil {
		t.Fatalf("remove isolated derived projection before rebuild: %v", err)
	}
	rebuilt, err := sink.Deliver(ctx, event)
	if err != nil || rebuilt == nil || *rebuilt != *receipt {
		t.Fatalf("rebuilt development projection=%+v err=%v", rebuilt, err)
	}

	assertDevelopmentServerProjectionReconciled(t, ctx, database, event, binding.MappingNo, binding.ProviderTaskNo)

	conflict := event
	conflict.CustomerNo = "10001"
	if conflictingReceipt, conflictErr := sink.Deliver(ctx, conflict); conflictErr == nil || conflictingReceipt != nil {
		t.Fatalf("conflicting development ownership was accepted: receipt=%+v err=%v", conflictingReceipt, conflictErr)
	}
	assertDevelopmentServerProjectionReconciled(t, ctx, database, event, binding.MappingNo, binding.ProviderTaskNo)
}

func seedDevelopmentServerProvisionReceipt(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	binding resourcecap.ServerEntitlementBinding,
	configurationSnapshotNo string,
) {
	t.Helper()
	_, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_provision_outcome_receipt(
 "tenant_id","receipt_no","task_no","service_no","operation_no","customer_no",
 "configuration_snapshot_no","status_code","request_digest","task_evidence_digest",
 "core_evidence_digest")
VALUES(0,?,?,?,?,?,?,'succeeded',?,?,?)`,
		"receipt_server_5", binding.ProviderTaskNo, binding.EntitlementNo,
		binding.SourceIntentNo, binding.CustomerNo, configurationSnapshotNo,
		strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64))
	if err != nil {
		t.Fatalf("seed development server provision receipt: %v", err)
	}
}

func assertDevelopmentServerProjectionReconciled(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	event entitlementcreatedcap.Event,
	mappingNo string,
	providerTaskNo string,
) {
	t.Helper()
	var evidence struct {
		ProjectionCount int    `orm:"projection_count"`
		MismatchCount   int    `orm:"mismatch_count"`
		ExternalRef     string `orm:"external_ref"`
	}
	err := database.GetScan(ctx, &evidence, `SELECT
 count(*) AS projection_count,
 count(*) FILTER (WHERE
   projection."customer_no"<>? OR projection."product_no"<>? OR
   projection."product_version_no"<>? OR projection."interface_instance_no"<>? OR
   projection."provider_plugin_id"<>? OR projection."plugin_release_version"<>? OR
   projection."fulfillment_contract_version"<>? OR projection."declaration_digest"<>? OR
   mapping."mapping_no"<>? OR mapping."status_code"<>'active' OR
   task."task_no"<>? OR task."status_code"<>'succeeded'
 ) AS mismatch_count,
 max(mapping."external_resource_ref") AS external_ref
FROM plugin_idc_resource_server_entitlement_projection projection
JOIN plugin_idc_resource_mapping mapping
  ON mapping."tenant_id"=projection."tenant_id"
 AND mapping."mapping_no"=projection."mapping_no"
 AND mapping."local_resource_no"=projection."local_resource_no"
JOIN plugin_idc_resource_task task
  ON task."tenant_id"=projection."tenant_id"
 AND task."task_no"=projection."provider_task_no"
 AND task."local_resource_no"=projection."local_resource_no"
WHERE projection."entitlement_no"=?`,
		event.CustomerNo, event.ProductNo, event.ProductVersionNo, event.InterfaceInstanceNo,
		event.PluginID, event.PluginReleaseVersion, event.ContractVersion, event.DeclarationDigest,
		mappingNo, providerTaskNo, event.EntitlementNo)
	if err != nil {
		t.Fatalf("read development server reconciliation evidence: %v", err)
	}
	if evidence.ProjectionCount != 1 || evidence.MismatchCount != 0 ||
		!strings.HasPrefix(evidence.ExternalRef, "upstream-server-") {
		t.Fatalf("development server reconciliation evidence=%+v", evidence)
	}
}
