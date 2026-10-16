// This file resolves current entitlement lifecycle routes exclusively from
// resource-owned ownership evidence. Callers cannot override the selected
// channel or pool, and every read rechecks the customer ownership boundary.

package lifecyclesource

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
)

func (s *Service) resolveEntitlementRoute(ctx context.Context, request lifecycleresourcecap.TaskRequest) (lifecycleresourcecap.TaskRequest, error) {
	if request.CustomerNo == "" {
		return request, nil
	}
	var route *struct{ ChannelNo, PoolNo string }
	if err := s.database.GetScan(ctx, &route, `SELECT mapping.channel_no,mapping.pool_no
FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping
  ON mapping.tenant_id=ownership.tenant_id AND mapping.mapping_no=ownership.mapping_no
 AND mapping.local_resource_no=ownership.local_resource_no
WHERE ownership.tenant_id=0 AND ownership.entitlement_no=? AND ownership.customer_no=?
  AND ownership.local_resource_no=ownership.entitlement_no
  AND mapping.status_code='active' AND mapping.external_resource_ref<>''`, request.ServiceNo, request.CustomerNo); err != nil {
		return lifecycleresourcecap.TaskRequest{}, gerror.Wrap(err, "resolve resource entitlement lifecycle ownership")
	}
	if route == nil || route.ChannelNo == "" || route.PoolNo == "" {
		return lifecycleresourcecap.TaskRequest{}, lifecycleresourcecap.ErrSourceUnavailable
	}
	request.ChannelNo, request.PoolNo = route.ChannelNo, route.PoolNo
	return request, nil
}
