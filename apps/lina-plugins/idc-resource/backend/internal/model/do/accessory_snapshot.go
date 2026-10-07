// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AccessorySnapshot is the golang structure of table plugin_idc_resource_accessory_snapshot for DAO operations like Where/Data.
type AccessorySnapshot struct {
	g.Meta              `orm:"table:plugin_idc_resource_accessory_snapshot, do:true"`
	Id                  any        //
	TenantId            any        //
	SnapshotNo          any        //
	MappingNo           any        //
	LocalResourceNo     any        //
	ChannelNo           any        //
	StatusCode          any        //
	SnapshotFingerprint any        //
	ItemCount           any        //
	ObservedAt          *time.Time //
	ExpiresAt           *time.Time //
	CreatedAt           *time.Time //
}
