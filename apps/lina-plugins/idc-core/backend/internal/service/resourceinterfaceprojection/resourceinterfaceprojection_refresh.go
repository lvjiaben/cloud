// This file refreshes only unused resource interface evidence under its core row
// lock and an owner callback sharing the same database transaction context.
package resourceinterfaceprojection

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"slices"
)

func (service *serviceImpl) RefreshUnused(ctx context.Context, projection resourceinterfaceprojectioncap.Projection, apply func(context.Context) error) error {
	if service == nil || service.database == nil || ctx == nil || ctx.Err() != nil || apply == nil || resourceinterfaceprojectioncap.ValidateProjection(projection) != nil {
		return gerror.New("resource interface refresh is unavailable")
	}
	return service.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var current *struct {
			OwnerID, ChannelRef, PluginID, ReleaseVersion, ReleaseDigest, DeclarationDigest string
			EverReferenced                                                                  bool
		}
		if err := service.database.GetScan(txCtx, &current, `SELECT resource_owner_id AS owner_id,resource_channel_ref AS channel_ref,plugin_id,plugin_release_version AS release_version,release_digest,declaration_digest,ever_referenced FROM plugin_idc_core_product_automation_interface_instance WHERE tenant_id=0 AND interface_instance_no=? FOR UPDATE`, projection.InterfaceInstanceNo); err != nil {
			return err
		}
		if current == nil || current.OwnerID != projection.OwnerID || current.ChannelRef != projection.ResourceChannelRef || current.PluginID != projection.PluginID {
			return gerror.New("resource interface refresh target changed")
		}
		if current.EverReferenced {
			return bizerr.NewCode(resourceinterfaceprojectioncap.CodeRefreshReferenced)
		}
		referenced, err := service.database.GetValue(txCtx, `SELECT
 EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_version_fulfillment_binding WHERE tenant_id=0 AND interface_instance_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_version_configuration WHERE tenant_id=0 AND interface_instance_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_core_catalog_import_contract WHERE tenant_id=0 AND interface_instance_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_import WHERE tenant_id=0 AND interface_instance_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND interface_instance_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND interface_instance_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND resource_owner_code=? AND channel_no=?)`,
			projection.InterfaceInstanceNo, projection.InterfaceInstanceNo, projection.InterfaceInstanceNo, projection.InterfaceInstanceNo, projection.InterfaceInstanceNo, projection.InterfaceInstanceNo, projection.OwnerID, projection.ResourceChannelRef)
		if err != nil {
			return err
		}
		if referenced.Bool() {
			return bizerr.NewCode(resourceinterfaceprojectioncap.CodeRefreshReferenced)
		}
		authorizedMethods := append([]string(nil), projection.AuthorizedMethods...)
		slices.Sort(authorizedMethods)
		methods, err := postgresTextArray(authorizedMethods)
		if err != nil {
			return err
		}
		if current.ReleaseVersion == projection.PluginReleaseVersion && current.ReleaseDigest == projection.ReleaseDigest && current.DeclarationDigest == projection.DeclarationDigest {
			return gerror.New("resource interface already uses this release")
		}
		if err = apply(txCtx); err != nil {
			return err
		}
		if _, err = service.database.Exec(txCtx, `UPDATE plugin_idc_core_product_automation_interface_instance SET plugin_release_version=?,contract_version=?,authorized_methods=CAST(? AS TEXT[]),release_digest=?,declaration_digest=?,version=version+1,changed_by=? WHERE tenant_id=0 AND interface_instance_no=?`, projection.PluginReleaseVersion, projection.ContractVersion, methods, projection.ReleaseDigest, projection.DeclarationDigest, projection.ActorUserID, projection.InterfaceInstanceNo); err != nil {
			return err
		}
		// Upsert sees matching new evidence and appends the owner revision projection.
		return service.Upsert(txCtx, projection)
	})
}
