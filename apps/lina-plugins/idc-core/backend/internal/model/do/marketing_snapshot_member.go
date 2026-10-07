// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSnapshotMember is the golang structure of table plugin_idc_core_marketing_snapshot_member for DAO operations like Where/Data.
type MarketingSnapshotMember struct {
	g.Meta     `orm:"table:plugin_idc_core_marketing_snapshot_member, do:true"`
	Id         any        //
	TenantId   any        //
	SnapshotNo any        //
	CustomerNo any        //
	CreatedAt  *time.Time //
}
