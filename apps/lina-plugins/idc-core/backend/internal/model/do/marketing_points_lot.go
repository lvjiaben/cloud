// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPointsLot is the golang structure of table plugin_idc_core_marketing_points_lot for DAO operations like Where/Data.
type MarketingPointsLot struct {
	g.Meta                  `orm:"table:plugin_idc_core_marketing_points_lot, do:true"`
	Id                      any        //
	TenantId                any        //
	LotNo                   any        //
	CustomerNo              any        //
	CampaignNo              *string    //
	CampaignTypeCode        *string    //
	SourceTypeCode          any        //
	SourceReference         any        //
	OriginalPoints          any        //
	AvailablePoints         any        //
	FrozenPoints            any        //
	ConsumedPoints          any        //
	ExpiredPoints           any        //
	ReversedPoints          any        //
	ReversalShortfallPoints any        // Cumulative refunded award points already consumed and therefore represented as account debt
	ExpiresAt               *time.Time //
	StatusCode              any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
