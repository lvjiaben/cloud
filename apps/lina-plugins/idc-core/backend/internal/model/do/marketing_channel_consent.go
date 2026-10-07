// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingChannelConsent is the golang structure of table plugin_idc_core_marketing_channel_consent for DAO operations like Where/Data.
type MarketingChannelConsent struct {
	g.Meta            `orm:"table:plugin_idc_core_marketing_channel_consent, do:true"`
	Id                any        //
	TenantId          any        //
	CustomerNo        any        //
	ContactNo         any        //
	ChannelCode       any        //
	StatusCode        any        //
	CurrentEventNo    any        //
	ConsentOccurredAt *time.Time //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
