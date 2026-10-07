// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPointsEntry is the golang structure of table plugin_idc_core_marketing_points_entry for DAO operations like Where/Data.
type MarketingPointsEntry struct {
	g.Meta            `orm:"table:plugin_idc_core_marketing_points_entry, do:true"`
	Id                any        //
	TenantId          any        //
	EntryNo           any        //
	CustomerNo        any        //
	LotNo             *string    //
	EntryTypeCode     any        //
	AvailableDelta    any        //
	FrozenDelta       any        //
	DebtDelta         any        //
	SourceTypeCode    any        //
	SourceReference   any        //
	EntryLineNo       any        //
	ReversalOfEntryNo *string    //
	CreatedAt         *time.Time //
}
