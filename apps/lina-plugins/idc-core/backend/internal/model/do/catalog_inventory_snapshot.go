// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogInventorySnapshot is the golang structure of table plugin_idc_core_catalog_inventory_snapshot for DAO operations like Where/Data.
type CatalogInventorySnapshot struct {
	g.Meta            `orm:"table:plugin_idc_core_catalog_inventory_snapshot, do:true"`
	Id                any        //
	TenantId          any        //
	SnapshotNo        any        //
	BindingNo         any        //
	SourceSnapshotNo  any        //
	SourceFingerprint any        //
	AvailabilityCode  any        //
	AvailableQuantity *int64     //
	SafetyMargin      any        //
	SellableQuantity  *int64     //
	ObservedAt        *time.Time //
	ExpiresAt         *time.Time //
	CreatedAt         *time.Time //
	SourceCompletedAt *time.Time // Completion time of the authoritative source snapshot used for this local projection
}
