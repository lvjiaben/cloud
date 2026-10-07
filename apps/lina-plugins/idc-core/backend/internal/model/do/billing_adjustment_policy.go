// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingAdjustmentPolicy is the golang structure of table plugin_idc_core_billing_adjustment_policy for DAO operations like Where/Data.
type BillingAdjustmentPolicy struct {
	g.Meta               `orm:"table:plugin_idc_core_billing_adjustment_policy, do:true"`
	Id                   any        //
	TenantId             any        //
	PolicyNo             any        //
	CurrencyCode         any        //
	ReviewThresholdMinor any        //
	Version              any        //
	ConfiguredBy         any        //
	ContentDigest        any        //
	CreatedAt            *time.Time //
}
