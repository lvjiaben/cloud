// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegment is the golang structure of table plugin_idc_core_marketing_segment for DAO operations like Where/Data.
type MarketingSegment struct {
	g.Meta                 `orm:"table:plugin_idc_core_marketing_segment, do:true"`
	Id                     any        //
	TenantId               any        //
	SegmentNo              any        //
	SegmentCode            any        //
	NameZhCn               any        //
	NameEnUs               any        //
	DescriptionZhCn        any        //
	DescriptionEnUs        any        //
	ModeCode               any        //
	RefreshIntervalMinutes *int       //
	DefinitionDigest       any        //
	StatusCode             any        //
	Version                any        //
	CreatedBy              any        //
	UpdatedBy              any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
}
