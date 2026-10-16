// This file supplies trusted aggregate lifecycle impact without exposing business details.
package businessimpact

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
)

// NewLazy binds the owner database lazily because source declarations load before startup.
func NewLazy(database func() gdb.DB) (pluginhost.SourcePluginBusinessImpactHandler, error) {
	if database == nil {
		return nil, gerror.New("business impact requires database factory")
	}
	return func(ctx context.Context, input pluginhost.PluginBusinessImpactInput) ([]pluginhost.PluginBusinessImpactItem, error) {
		if ctx == nil || ctx.Err() != nil || strings.TrimSpace(input.TargetPluginID) == "" {
			return nil, gerror.New("business impact requires database and target")
		}
		db := database()
		if db == nil {
			return nil, gerror.New("business impact database unavailable")
		}
		var rows []struct {
			Category pluginhost.BusinessImpactCategory
			Total    int64
			Blocking int64
		}
		if err := db.GetScan(ctx, &rows, impactSQL, input.TargetPluginID); err != nil {
			return nil, err
		}
		result := make([]pluginhost.PluginBusinessImpactItem, 0, len(rows))
		for _, row := range rows {
			result = append(result, pluginhost.PluginBusinessImpactItem{Category: row.Category, TotalCount: row.Total, BlockingCount: row.Blocking})
		}
		return result, nil
	}, nil
}

// impactSQL projects a fixed set of owner aggregates; historical rows remain visible after draining.
const impactSQL = `WITH channels AS (SELECT channel_no FROM plugin_idc_resource_channel WHERE tenant_id=0 AND provider_plugin_id=?),
tasks AS (
 SELECT t.status_code FROM plugin_idc_resource_task t JOIN channels c ON c.channel_no=t.channel_no WHERE t.tenant_id=0
 UNION ALL
 SELECT p.status_code FROM plugin_idc_resource_principal_task p JOIN channels c ON c.channel_no=p.channel_no WHERE p.tenant_id=0
),
services AS (SELECT m.status_code FROM plugin_idc_resource_mapping m JOIN channels c ON c.channel_no=m.channel_no WHERE m.tenant_id=0),
leases AS (SELECT l.lease_until FROM plugin_idc_resource_channel_invocation_lease l JOIN channels c ON c.channel_no=l.channel_no WHERE l.tenant_id=0)
SELECT 'in_flight_tasks' AS category,COUNT(*) AS total,COUNT(*) FILTER (WHERE status_code NOT IN ('succeeded','failed','manual_takeover','canceled')) AS blocking FROM tasks
UNION ALL
SELECT 'active_services',COUNT(*),COUNT(*) FILTER (WHERE status_code<>'released') FROM services
UNION ALL
SELECT 'pending_historical_actions',COUNT(*),COUNT(*) FILTER (WHERE lease_until>CURRENT_TIMESTAMP) FROM leases`
