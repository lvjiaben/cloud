// This file binds the paid entitlement identity to the exact resource outcome.
// Corrected attempts retain the paid interface in owner metadata while the
// local server projection records the actual executed channel interface.

package serverownership

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const EntitlementOwnerID = "idc-resource"

type entitlementEvidence struct {
	SaleImportProtocol   productfulfillmentcap.SaleImportProtocol `orm:"sale_import_protocol"`
	InterfaceInstanceNo  string                                   `orm:"interface_instance_no"`
	MappingNo            string                                   `orm:"mapping_no"`
	ProviderTaskNo       string                                   `orm:"provider_task_no"`
	LocalResourceNo      string                                   `orm:"local_resource_no"`
	ReleaseDigest        string                                   `orm:"release_digest"`
	PluginID             string                                   `orm:"plugin_id"`
	PluginReleaseVersion string                                   `orm:"plugin_release_version"`
	DeclarationDigest    string                                   `orm:"declaration_digest"`
	CoreEvidenceDigest   string                                   `orm:"core_evidence_digest"`
}

// EntitlementCreatedSink consumes only events routed to the idc-resource
// business owner. It resolves the server through a succeeded, local,
// append-only provision receipt and formal task/mapping columns.
type EntitlementCreatedSink struct {
	database gdb.DB
	owners   *Service
}

func NewEntitlementCreatedSink(database gdb.DB) (*EntitlementCreatedSink, error) {
	owners, err := New(database)
	if err != nil {
		return nil, err
	}
	return &EntitlementCreatedSink{database: database, owners: owners}, nil
}

func (sink *EntitlementCreatedSink) Deliver(
	ctx context.Context,
	event entitlementcreatedcap.Event,
) (*entitlementcreatedcap.Receipt, error) {
	if sink == nil || sink.database == nil || sink.owners == nil || ctx == nil ||
		ctx.Err() != nil || !entitlementcreatedcap.ValidateEvent(event) ||
		event.OwnerID != EntitlementOwnerID {
		return nil, gerror.New("resource entitlement event is unavailable")
	}
	var evidenceRows []entitlementEvidence
	// Paid evidence belongs to the original interface. The succeeded attempt
	// owns a separate, compatible execution identity; requiring its declaration
	// digest to equal the paid one would lose a legitimately corrected server.
	if err := sink.database.GetScan(ctx, &evidenceRows, `
SELECT COALESCE(task.owner_metadata->>'saleImportProtocol','') sale_import_protocol,mapping.mapping_no,task.task_no provider_task_no,
       task.local_resource_no,channel.provider_release_digest release_digest,
       receipt.core_evidence_digest,interface_binding.interface_instance_no,
       channel.provider_plugin_id plugin_id,
       channel.provider_release_version plugin_release_version,
       interface_binding.product_declaration_digest declaration_digest
FROM plugin_idc_resource_provision_outcome_receipt receipt
JOIN plugin_idc_resource_task task
  ON task.tenant_id=receipt.tenant_id AND task.task_no=receipt.task_no
JOIN plugin_idc_resource_mapping mapping
  ON mapping.tenant_id=task.tenant_id
 AND mapping.local_resource_no=task.local_resource_no
 AND mapping.channel_no=task.channel_no
JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id=task.tenant_id AND channel.channel_no=task.channel_no
JOIN plugin_idc_resource_interface_channel_binding interface_binding
  ON interface_binding.tenant_id=channel.tenant_id
 AND interface_binding.channel_no=channel.channel_no
JOIN plugin_idc_resource_interface_channel_binding purchased_binding
  ON purchased_binding.tenant_id=task.tenant_id
 AND purchased_binding.interface_instance_no=?
JOIN plugin_idc_resource_channel purchased_channel
  ON purchased_channel.tenant_id=purchased_binding.tenant_id
 AND purchased_channel.channel_no=purchased_binding.channel_no
WHERE receipt.tenant_id=0 AND receipt.status_code='succeeded'
  AND receipt.operation_no=?
  AND receipt.customer_no=? AND receipt.configuration_snapshot_no=?
  AND task.capability_code=? AND task.status_code='succeeded'
  AND mapping.status_code='active'
  AND channel.provider_runtime_type='dynamic'
  AND COALESCE(task.owner_metadata->>'purchasedInterfaceInstanceNo',interface_binding.interface_instance_no)=?
  AND interface_binding.interface_instance_no=COALESCE(task.owner_metadata->'recoveryConfiguration'->>'interfaceInstanceNo',?)
  AND purchased_binding.product_plugin_id=?
  AND purchased_binding.product_plugin_release_version=?
  AND purchased_binding.product_declaration_digest=?
  AND purchased_binding.product_contract_version=?
  AND channel.provider_id=purchased_channel.provider_id
  AND channel.provider_plugin_id=purchased_channel.provider_plugin_id
  AND channel.provider_contract_version=purchased_channel.provider_contract_version
  AND interface_binding.product_plugin_id=channel.provider_plugin_id
  AND interface_binding.product_plugin_release_version=channel.provider_release_version
  AND interface_binding.product_release_digest=channel.provider_release_digest
  AND interface_binding.product_contract_version=purchased_binding.product_contract_version
LIMIT 2`, event.InterfaceInstanceNo, event.IntentNo, event.CustomerNo,
		event.ConfigurationSnapshotNo, resourcecap.CapabilityProvision,
		event.InterfaceInstanceNo, event.InterfaceInstanceNo,
		event.PluginID, event.PluginReleaseVersion, event.DeclarationDigest, event.ContractVersion); err != nil {
		return nil, gerror.Wrap(err, "resolve resource entitlement evidence")
	}
	if len(evidenceRows) != 1 {
		return nil, gerror.New("resource entitlement evidence does not match")
	}
	evidence := &evidenceRows[0]
	binding := resourcecap.ServerEntitlementBinding{
		Version:       resourcecap.ServerEntitlementProjectionVersion,
		EntitlementNo: event.EntitlementNo, SourceIntentNo: event.IntentNo,
		CustomerNo: event.CustomerNo, ProductNo: event.ProductNo,
		ProductVersionNo: event.ProductVersionNo, ProductNameZhCN: event.ProductNameZhCN,
		ProductNameEnUS: event.ProductNameEnUS, LocalResourceNo: evidence.LocalResourceNo,
		MappingNo: evidence.MappingNo, ProviderTaskNo: evidence.ProviderTaskNo,
		InterfaceInstanceNo: evidence.InterfaceInstanceNo,
		ProviderPluginID:    evidence.PluginID, PluginReleaseVersion: evidence.PluginReleaseVersion,
		FulfillmentContractVersion: event.ContractVersion,
		ServerContractVersion:      resourcecap.ServerFulfillmentContractVersion,
		ReleaseDigest:              evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest,
	}
	if evidence.SaleImportProtocol.NativePurchase() {
		if !strings.HasPrefix(evidence.LocalResourceNo, "NAT_") {
			return nil, gerror.New("native purchase entitlement identity is invalid")
		}
	} else {
		if evidence.SaleImportProtocol != "" || strings.HasPrefix(evidence.LocalResourceNo, "NAT_") {
			return nil, gerror.New("resource entitlement protocol is invalid")
		}
		if err := sink.owners.Bind(ctx, binding); err != nil {
			return nil, err
		}
	}
	digest := sha256.Sum256([]byte(
		"idc-resource/entitlement-created-receipt/v1\x00" + event.EventNo + "\x00" +
			event.EntitlementNo + "\x00" + evidence.MappingNo + "\x00" +
			evidence.ProviderTaskNo + "\x00" + evidence.CoreEvidenceDigest,
	))
	return &entitlementcreatedcap.Receipt{
		Version:        entitlementcreatedcap.ContractVersion,
		EventNo:        event.EventNo,
		OwnerReference: evidence.MappingNo,
		EvidenceDigest: hex.EncodeToString(digest[:]),
	}, nil
}
