// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegmentSnapshot is the golang structure of table plugin_idc_core_marketing_segment_snapshot for DAO operations like Where/Data.
type MarketingSegmentSnapshot struct {
	g.Meta           `orm:"table:plugin_idc_core_marketing_segment_snapshot, do:true"`
	Id               any        //
	TenantId         any        //
	SnapshotNo       any        //
	SegmentNo        any        //
	SegmentVersion   any        //
	DefinitionDigest any        //
	StatusCode       any        //
	MemberCount      any        //
	ContentDigest    *string    //
	StartedAt        *time.Time //
	CompletedAt      *time.Time //
	CreatedAt        *time.Time //
}
