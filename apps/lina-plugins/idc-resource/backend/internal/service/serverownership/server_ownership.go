// Package serverownership persists the resource-owner side of one frozen
// generic entitlement-to-server binding.
package serverownership

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type Service struct{ database gdb.DB }

// New creates the ownership writer using the startup-shared database handle.
func New(database gdb.DB) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource server ownership database is required")
	}
	return &Service{database: database}, nil
}

type projectionRow struct {
	EntitlementNo              string `orm:"entitlement_no"`
	SourceIntentNo             string `orm:"source_intent_no"`
	CustomerNo                 string `orm:"customer_no"`
	ProductNo                  string `orm:"product_no"`
	ProductVersionNo           string `orm:"product_version_no"`
	ProductNameZhCN            string `orm:"product_name_zh_cn"`
	ProductNameEnUS            string `orm:"product_name_en_us"`
	LocalResourceNo            string `orm:"local_resource_no"`
	MappingNo                  string `orm:"mapping_no"`
	ProviderTaskNo             string `orm:"provider_task_no"`
	InterfaceInstanceNo        string `orm:"interface_instance_no"`
	ProviderPluginID           string `orm:"provider_plugin_id"`
	PluginReleaseVersion       string `orm:"plugin_release_version"`
	FulfillmentContractVersion string `orm:"fulfillment_contract_version"`
	ServerContractVersion      string `orm:"server_contract_version"`
	ReleaseDigest              string `orm:"release_digest"`
	DeclarationDigest          string `orm:"declaration_digest"`
}

// Bind creates or reuses one exact projection. It proves the resource-side
// task and mapping in one transaction and never reads another plugin's tables.
func (service *Service) Bind(
	ctx context.Context,
	binding resourcecap.ServerEntitlementBinding,
) error {
	if service == nil || service.database == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource server ownership service is unavailable")
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	return service.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Exec(`
INSERT INTO plugin_idc_resource_server_entitlement_projection(
    tenant_id,entitlement_no,source_intent_no,customer_no,product_no,product_version_no,
    product_name_zh_cn,product_name_en_us,local_resource_no,
    mapping_no,provider_task_no,interface_instance_no,provider_plugin_id,
    plugin_release_version,fulfillment_contract_version,server_contract_version,
    release_digest,declaration_digest
)
SELECT 0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?
FROM plugin_idc_resource_mapping mapping
JOIN plugin_idc_resource_task task
  ON task.tenant_id=mapping.tenant_id
 AND task.local_resource_no=mapping.local_resource_no
 AND task.channel_no=mapping.channel_no
JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id=task.tenant_id
 AND channel.channel_no=task.channel_no
JOIN plugin_idc_resource_interface_channel_binding interface_binding
  ON interface_binding.tenant_id=channel.tenant_id
 AND interface_binding.channel_no=channel.channel_no
WHERE mapping.tenant_id=0 AND mapping.mapping_no=?
  AND mapping.local_resource_no=? AND mapping.status_code='active'
  AND task.task_no=? AND task.capability_code=? AND task.status_code='succeeded'
  AND channel.provider_runtime_type='dynamic'
  AND channel.provider_plugin_id=?
  AND channel.provider_release_version=?
  AND channel.provider_release_digest=?
  AND interface_binding.interface_instance_no=?
  AND interface_binding.product_declaration_digest=?
ON CONFLICT(entitlement_no) DO UPDATE
SET product_no=EXCLUDED.product_no,
    product_version_no=EXCLUDED.product_version_no,
    product_name_zh_cn=EXCLUDED.product_name_zh_cn,
    product_name_en_us=EXCLUDED.product_name_en_us,
    updated_at=CURRENT_TIMESTAMP
WHERE (plugin_idc_resource_server_entitlement_projection.product_no IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_no=EXCLUDED.product_no)
  AND (plugin_idc_resource_server_entitlement_projection.product_version_no IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_version_no=EXCLUDED.product_version_no)
  AND (plugin_idc_resource_server_entitlement_projection.product_name_zh_cn IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_name_zh_cn=EXCLUDED.product_name_zh_cn)
  AND (plugin_idc_resource_server_entitlement_projection.product_name_en_us IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_name_en_us=EXCLUDED.product_name_en_us)
  AND (plugin_idc_resource_server_entitlement_projection.product_no IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_version_no IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_name_zh_cn IS NULL OR
       plugin_idc_resource_server_entitlement_projection.product_name_en_us IS NULL)`,
			binding.EntitlementNo,
			binding.SourceIntentNo,
			binding.CustomerNo,
			binding.ProductNo,
			binding.ProductVersionNo,
			binding.ProductNameZhCN,
			binding.ProductNameEnUS,
			binding.LocalResourceNo,
			binding.MappingNo,
			binding.ProviderTaskNo,
			binding.InterfaceInstanceNo,
			binding.ProviderPluginID,
			binding.PluginReleaseVersion,
			binding.FulfillmentContractVersion,
			binding.ServerContractVersion,
			binding.ReleaseDigest,
			binding.DeclarationDigest,
			binding.MappingNo,
			binding.LocalResourceNo,
			binding.ProviderTaskNo,
			resourcecap.CapabilityProvision,
			binding.ProviderPluginID,
			binding.PluginReleaseVersion,
			binding.ReleaseDigest,
			binding.InterfaceInstanceNo,
			binding.DeclarationDigest,
		)
		if err != nil {
			return gerror.Wrap(err, "bind resource server entitlement ownership")
		}
		var persisted *projectionRow
		if err = tx.GetScan(&persisted, `
SELECT entitlement_no,source_intent_no,customer_no,product_no,product_version_no,
       product_name_zh_cn,product_name_en_us,local_resource_no,mapping_no,
       provider_task_no,interface_instance_no,provider_plugin_id,
       plugin_release_version,fulfillment_contract_version,server_contract_version,
       release_digest,declaration_digest
FROM plugin_idc_resource_server_entitlement_projection
WHERE tenant_id=0 AND entitlement_no=?
LIMIT 1 FOR UPDATE`, binding.EntitlementNo); err != nil {
			return gerror.Wrap(err, "load resource server entitlement ownership")
		}
		if persisted == nil || !sameProjection(persisted, binding) {
			return gerror.New("resource server entitlement ownership evidence does not match")
		}
		return nil
	})
}

func sameProjection(row *projectionRow, binding resourcecap.ServerEntitlementBinding) bool {
	return row.EntitlementNo == binding.EntitlementNo &&
		row.SourceIntentNo == binding.SourceIntentNo &&
		row.CustomerNo == binding.CustomerNo &&
		row.ProductNo == binding.ProductNo &&
		row.ProductVersionNo == binding.ProductVersionNo &&
		row.ProductNameZhCN == binding.ProductNameZhCN &&
		row.ProductNameEnUS == binding.ProductNameEnUS &&
		row.LocalResourceNo == binding.LocalResourceNo &&
		row.MappingNo == binding.MappingNo &&
		row.ProviderTaskNo == binding.ProviderTaskNo &&
		row.InterfaceInstanceNo == binding.InterfaceInstanceNo &&
		row.ProviderPluginID == binding.ProviderPluginID &&
		row.PluginReleaseVersion == binding.PluginReleaseVersion &&
		row.FulfillmentContractVersion == binding.FulfillmentContractVersion &&
		row.ServerContractVersion == binding.ServerContractVersion &&
		row.ReleaseDigest == binding.ReleaseDigest &&
		row.DeclarationDigest == binding.DeclarationDigest
}
