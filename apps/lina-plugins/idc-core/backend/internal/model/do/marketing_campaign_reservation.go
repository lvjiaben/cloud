// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingCampaignReservation is the golang structure of table plugin_idc_core_marketing_campaign_reservation for DAO operations like Where/Data.
type MarketingCampaignReservation struct {
	g.Meta                `orm:"table:plugin_idc_core_marketing_campaign_reservation, do:true"`
	Id                    any        //
	TenantId              any        //
	ReservationNo         any        //
	CampaignNo            any        //
	BenefitNo             any        //
	CustomerNo            any        //
	QuoteReference        any        //
	RequestSnapshotDigest any        //
	CurrencyCode          any        //
	ReservedUnits         any        //
	QuotedValueMinor      any        //
	BenefitSnapshotDigest any        //
	StatusCode            any        //
	ExpiresAt             *time.Time //
	ConsumedAt            *time.Time //
	ReleasedAt            *time.Time //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
	ReversedAt            *time.Time //
}
