// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditPolicyRevision is the golang structure of table plugin_idc_core_credit_policy_revision for DAO operations like Where/Data.
type CreditPolicyRevision struct {
	g.Meta                   `orm:"table:plugin_idc_core_credit_policy_revision, do:true"`
	Id                       any        //
	TenantId                 any        //
	PolicyNo                 any        //
	PolicyVersion            any        //
	CurrencyCode             any        //
	MaximumLimitMinor        any        //
	DualReviewThresholdMinor any        //
	BillingDay               any        //
	RepaymentPeriodDays      any        //
	GracePeriodDays          any        //
	DailyLateFeeBasisPoints  any        //
	MaximumLateFeeMinor      any        //
	ConfiguredBy             any        //
	ContentDigest            any        //
	CreatedAt                *time.Time //
}
