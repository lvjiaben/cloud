// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AccessorySnapshotItem is the golang structure of table plugin_idc_resource_accessory_snapshot_item for DAO operations like Where/Data.
type AccessorySnapshotItem struct {
	g.Meta        `orm:"table:plugin_idc_resource_accessory_snapshot_item, do:true"`
	Id            any        //
	TenantId      any        //
	SnapshotNo    any        //
	ObjectNo      any        // Local opaque object number exposed to callers instead of Provider identifiers
	AccessoryKind any        //
	ExternalRef   any        // Resource-owner private Provider reference; never returned, logged, or audited
	DisplayName   any        //
	StateCode     any        //
	Projection    any        //
	ObservedAt    *time.Time //
	CreatedAt     *time.Time //
}
