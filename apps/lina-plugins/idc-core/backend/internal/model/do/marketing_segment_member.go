// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegmentMember is the golang structure of table plugin_idc_core_marketing_segment_member for DAO operations like Where/Data.
type MarketingSegmentMember struct {
	g.Meta         `orm:"table:plugin_idc_core_marketing_segment_member, do:true"`
	Id             any        //
	TenantId       any        //
	SegmentNo      any        //
	CustomerNo     any        //
	SourceCode     any        //
	SegmentVersion any        //
	StatusCode     any        //
	EvaluatedAt    *time.Time //
	RevokedAt      *time.Time //
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
}
