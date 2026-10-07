// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingBenefitRule is the golang structure of table plugin_idc_core_marketing_benefit_rule for DAO operations like Where/Data.
type MarketingBenefitRule struct {
	g.Meta               `orm:"table:plugin_idc_core_marketing_benefit_rule, do:true"`
	Id                   any        //
	TenantId             any        //
	BenefitNo            any        //
	CampaignNo           any        //
	CampaignTypeCode     any        //
	BenefitTypeCode      any        //
	CurrencyCode         *string    //
	AmountMinor          *int64     //
	RateBasisPoints      *int       //
	MaximumDiscountMinor *int64     //
	MinimumSpendMinor    *int64     //
	PointsQuantity       *int64     //
	PurchasePhaseCode    any        //
	CycleScopeCode       any        //
	CycleCount           *int       //
	StatusCode           any        //
	Version              any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
