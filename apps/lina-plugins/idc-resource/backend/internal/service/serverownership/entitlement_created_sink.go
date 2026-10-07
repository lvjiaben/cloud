package serverownership

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const EntitlementOwnerID = "idc-resource"

type entitlementEvidence struct {
	MappingNo          string `orm:"mapping_no"`
	ProviderTaskNo     string `orm:"provider_task_no"`
	LocalResourceNo    string `orm:"local_resource_no"`
	ReleaseDigest      string `orm:"release_digest"`
	CoreEvidenceDigest string `orm:"core_evidence_digest"`
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
	if err := sink.database.GetScan(ctx, &evidenceRows, `
SELECT mapping.mapping_no,task.task_no provider_task_no,
       task.local_resource_no,channel.provider_release_digest release_digest,
       receipt.core_evidence_digest
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
WHERE receipt.tenant_id=0 AND receipt.status_code='succeeded'
  AND receipt.operation_no=?
  AND receipt.customer_no=? AND receipt.configuration_snapshot_no=?
  AND task.capability_code=? AND task.status_code='succeeded'
  AND mapping.status_code='active'
  AND channel.provider_runtime_type='dynamic'
  AND interface_binding.interface_instance_no=?
  AND interface_binding.product_plugin_id=?
  AND interface_binding.product_plugin_release_version=?
  AND interface_binding.product_declaration_digest=?
LIMIT 2`, event.IntentNo, event.CustomerNo,
		event.ConfigurationSnapshotNo, resourcecap.CapabilityProvision,
		event.InterfaceInstanceNo,
		event.PluginID, event.PluginReleaseVersion, event.DeclarationDigest); err != nil {
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
		InterfaceInstanceNo: event.InterfaceInstanceNo,
		ProviderPluginID:    event.PluginID, PluginReleaseVersion: event.PluginReleaseVersion,
		FulfillmentContractVersion: event.ContractVersion,
		ServerContractVersion:      resourcecap.ServerFulfillmentContractVersion,
		ReleaseDigest:              evidence.ReleaseDigest, DeclarationDigest: event.DeclarationDigest,
	}
	if err := sink.owners.Bind(ctx, binding); err != nil {
		return nil, err
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
