// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderItemInventorySnapshot is the golang structure of table plugin_idc_core_commerce_order_item_inventory_snapshot for DAO operations like Where/Data.
type CommerceOrderItemInventorySnapshot struct {
	g.Meta           `orm:"table:plugin_idc_core_commerce_order_item_inventory_snapshot, do:true"`
	OrderItemNo      any        //
	QuoteItemNo      any        //
	VersionNo        any        //
	TrackingEnabled  any        //
	InventoryVersion any        //
	SellableQuantity any        //
	ContentDigest    any        //
	CreatedAt        *time.Time //
}
