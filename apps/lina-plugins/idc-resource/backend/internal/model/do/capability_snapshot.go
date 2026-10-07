// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CapabilitySnapshot is the golang structure of table plugin_idc_resource_capability_snapshot for DAO operations like Where/Data.
type CapabilitySnapshot struct {
	g.Meta         `orm:"table:plugin_idc_resource_capability_snapshot, do:true"`
	Id             any        //
	TenantId       any        //
	SnapshotNo     any        //
	ChannelNo      any        //
	AdapterVersion any        //
	StatusCode     any        //
	ItemCount      any        //
	ObservedAt     *time.Time //
	CompletedAt    *time.Time //
	CreatedAt      *time.Time //
}
