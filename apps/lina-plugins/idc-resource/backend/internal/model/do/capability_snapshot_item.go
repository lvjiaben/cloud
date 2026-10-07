// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CapabilitySnapshotItem is the golang structure of table plugin_idc_resource_capability_snapshot_item for DAO operations like Where/Data.
type CapabilitySnapshotItem struct {
	g.Meta            `orm:"table:plugin_idc_resource_capability_snapshot_item, do:true"`
	Id                any        //
	TenantId          any        //
	SnapshotNo        any        //
	CapabilityCode    any        //
	SupportCode       any        //
	Constraints       any        //
	UnsupportedReason any        //
	CreatedAt         *time.Time //
}
