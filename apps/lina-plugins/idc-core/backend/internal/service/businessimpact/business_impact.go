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
const impactSQL = `WITH target AS (SELECT ?::text AS plugin_id),
bindings AS (
 SELECT b.version_no,p.status_code AS product_status,v.status_code AS version_status,p.current_version_no,p.deleted_at
 FROM plugin_idc_core_catalog_product_version_fulfillment_binding b
 JOIN target x ON x.plugin_id=b.plugin_id
 JOIN plugin_idc_core_catalog_product_version v ON v.tenant_id=b.tenant_id AND v.version_no=b.version_no
 JOIN plugin_idc_core_catalog_product p ON p.tenant_id=v.tenant_id AND p.product_no=v.product_no
 WHERE b.tenant_id=0
),
intents AS (SELECT i.* FROM plugin_idc_core_product_fulfillment_intent i JOIN target x ON x.plugin_id=i.plugin_id WHERE i.tenant_id=0),
services AS (SELECT e.* FROM plugin_idc_core_service_entitlement e JOIN target x ON x.plugin_id=e.plugin_id WHERE e.tenant_id=0),
orders AS (SELECT o.* FROM plugin_idc_core_commerce_order o WHERE o.tenant_id=0 AND EXISTS (
 SELECT 1 FROM plugin_idc_core_commerce_order_item i JOIN bindings b ON b.version_no=i.version_no WHERE i.tenant_id=o.tenant_id AND i.order_no=o.order_no)),
actions AS (
 SELECT a.status_code NOT IN ('succeeded','failed') AS blocking FROM plugin_idc_core_managed_service_action_task a JOIN target x ON x.plugin_id=a.plugin_id WHERE a.tenant_id=0
 UNION ALL
 SELECT o.status_code NOT IN ('succeeded','failed','cancelled') FROM plugin_idc_core_business_operation o JOIN services e ON e.entitlement_no=o.subject_no AND e.tenant_id=o.tenant_id WHERE o.tenant_id=0
 UNION ALL
 SELECT o.status_code IN ('pending_payment','payment_processing','paid','fulfilling','partially_fulfilled','refund_pending','manual_review') FROM orders o
 UNION ALL
 SELECT c.status_code<>'completed' FROM plugin_idc_core_commerce_order_cancellation c JOIN orders o ON o.order_no=c.order_no AND o.tenant_id=c.tenant_id WHERE c.tenant_id=0
 UNION ALL
 SELECT r.status_code NOT IN ('succeeded','failed','rejected') FROM plugin_idc_core_refund_request r JOIN orders o ON o.order_no=r.order_no AND o.tenant_id=r.tenant_id WHERE r.tenant_id=0
 UNION ALL
 SELECT l.released_at IS NULL FROM plugin_idc_core_refund_entitlement_lock l JOIN services e ON e.entitlement_no=l.entitlement_no AND e.tenant_id=l.tenant_id WHERE l.tenant_id=0
)
SELECT 'selling_product_versions' AS category,COUNT(*) AS total,COUNT(*) FILTER (WHERE deleted_at IS NULL AND product_status='active' AND version_status='published' AND current_version_no=version_no) AS blocking FROM bindings
UNION ALL
SELECT 'in_flight_tasks',COUNT(*),COUNT(*) FILTER (WHERE status_code NOT IN ('completed','incomplete','canceled')) FROM intents
UNION ALL
SELECT 'active_services',COUNT(*),COUNT(*) FILTER (WHERE status_code NOT IN ('terminated','cancelled')) FROM services
UNION ALL
SELECT 'pending_historical_actions',COUNT(*),COUNT(*) FILTER (WHERE blocking) FROM actions`
