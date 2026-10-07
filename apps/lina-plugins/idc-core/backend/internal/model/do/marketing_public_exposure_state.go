// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPublicExposureState is the golang structure of table plugin_idc_core_marketing_public_exposure_state for DAO operations like Where/Data.
type MarketingPublicExposureState struct {
	g.Meta          `orm:"table:plugin_idc_core_marketing_public_exposure_state, do:true"`
	Id              any        //
	TenantId        any        //
	DeviceDigest    any        //
	CampaignNo      any        //
	CurrencyCode    any        //
	WindowStartedAt *time.Time //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
