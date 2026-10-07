// Package serviceguard contains small, owner-local conflict checks shared by
// service Saga entry points. It never mutates a business aggregate.
package serviceguard

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// HasActiveRefundLock reports whether a submitted refund still freezes the
// service. Callers must already hold the service row lock so refund submission
// and the competing operation serialize in one deterministic order.
func HasActiveRefundLock(ctx context.Context, database gdb.DB, serviceNo string) (bool, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || serviceNo == "" ||
		serviceNo != strings.TrimSpace(serviceNo) || len(serviceNo) > 64 {
		return false, gerror.New("service refund guard input is invalid")
	}
	value, err := database.GetValue(ctx, `SELECT EXISTS (
SELECT 1
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_commerce_fulfillment_unit fulfillment
  ON fulfillment.tenant_id=service.tenant_id
 AND fulfillment.fulfillment_no=service.source_fulfillment_no
JOIN plugin_idc_core_service_entitlement entitlement
  ON entitlement.tenant_id=service.tenant_id
 AND entitlement.order_item_no=service.order_item_no
 AND entitlement.unit_sequence=fulfillment.unit_sequence
JOIN plugin_idc_core_refund_entitlement_lock entitlement_lock
  ON entitlement_lock.tenant_id=entitlement.tenant_id
 AND entitlement_lock.entitlement_no=entitlement.entitlement_no
 AND entitlement_lock.released_at IS NULL
WHERE service.tenant_id=0 AND service.service_no=?
)`, serviceNo)
	if err != nil {
		return false, gerror.Wrap(err, "check active entitlement refund lock for server service")
	}
	return value.Bool(), nil
}
