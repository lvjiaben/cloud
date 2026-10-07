// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingValueEntry is the golang structure of table plugin_idc_core_marketing_value_entry for DAO operations like Where/Data.
type MarketingValueEntry struct {
	g.Meta              `orm:"table:plugin_idc_core_marketing_value_entry, do:true"`
	Id                  any        //
	TenantId            any        //
	EntryNo             any        //
	GrantNo             any        //
	CustomerNo          any        //
	CurrencyCode        any        //
	EntryTypeCode       any        //
	PendingDeltaMinor   any        //
	AvailableDeltaMinor any        //
	FrozenDeltaMinor    any        //
	ConsumedDeltaMinor  any        //
	ExpiredDeltaMinor   any        //
	ReversedDeltaMinor  any        //
	SourceTypeCode      any        //
	SourceReference     any        //
	ReversalOfEntryNo   *string    //
	CreatedAt           *time.Time //
}
