// This file creates one private direct route only when a bound channel has no pools.
// Merchant-defined pools, including disabled pools, always retain routing authority.
package provisionsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

const directPoolSelector = `{"provision":{"mode":"direct"}}`

// ensureDefaultInterfacePool serializes against channel configuration writes;
// neither this route nor its selector copies or reads upstream credentials.
func (s *Service) ensureDefaultInterfacePool(ctx context.Context, interfaceNo, revisionNo string) error {
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var channel *entity.Channel
		err := s.database.Model(do.Channel{}).Ctx(txCtx).As("channel").InnerJoin("plugin_idc_resource_interface_channel_binding binding", "binding.tenant_id=channel.tenant_id AND binding.channel_no=channel.channel_no").InnerJoin("plugin_idc_resource_interface_configuration_revision revision", "revision.tenant_id=binding.tenant_id AND revision.interface_instance_no=binding.interface_instance_no").Where("channel.tenant_id", 0).Where("binding.interface_instance_no", interfaceNo).Where("revision.revision_no", revisionNo).Where("channel.status_code", "enabled").Where("channel.provider_runtime_type", "dynamic").Fields("channel.channel_no,channel.display_name").LockUpdate().Scan(&channel)
		if err != nil {
			return err
		}
		if channel == nil {
			return provisionresourcecap.ErrCapabilityUnavailable
		}
		count, err := s.database.Model(do.Pool{}).Ctx(txCtx).Where(do.Pool{TenantId: 0, ChannelNo: channel.ChannelNo}).Count()
		if err != nil || count > 0 {
			return err
		}
		digest := sha256.Sum256([]byte("direct-interface-route/v1\x00" + channel.ChannelNo))
		_, err = s.database.Model(do.Pool{}).Ctx(txCtx).Data(do.Pool{TenantId: 0, PoolNo: "POL_" + hex.EncodeToString(digest[:16]), ChannelNo: channel.ChannelNo, DisplayName: channel.DisplayName, ResourceKind: "virtual_server", StatusCode: "enabled", RoutingPriority: 100, RoutingWeight: 100, Selector: directPoolSelector, Version: 1}).Insert()
		return err
	})
}
