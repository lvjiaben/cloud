// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingCampaign is the golang structure of table plugin_idc_core_marketing_campaign for DAO operations like Where/Data.
type MarketingCampaign struct {
	g.Meta                `orm:"table:plugin_idc_core_marketing_campaign, do:true"`
	Id                    any        //
	TenantId              any        //
	CampaignNo            any        //
	CampaignCode          any        //
	NameZhCn              any        //
	NameEnUs              any        //
	DescriptionZhCn       any        //
	DescriptionEnUs       any        //
	CampaignTypeCode      any        //
	TargetSegmentNo       *string    //
	TargetSnapshotNo      *string    //
	StartsAt              *time.Time //
	EndsAt                *time.Time //
	Priority              any        //
	StackPolicyCode       any        //
	StackGroupCode        any        //
	GlobalQuotaLimit      *int64     //
	GlobalReservedCount   any        //
	GlobalConsumedCount   any        //
	PerCustomerQuotaLimit *int       //
	StatusCode            any        //
	Version               any        //
	CreatedBy             any        //
	UpdatedBy             any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
	IsNewcomerExclusive   any        //
}
