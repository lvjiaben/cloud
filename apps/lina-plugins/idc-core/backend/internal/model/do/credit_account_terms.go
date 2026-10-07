// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditAccountTerms is the golang structure of table plugin_idc_core_credit_account_terms for DAO operations like Where/Data.
type CreditAccountTerms struct {
	g.Meta                  `orm:"table:plugin_idc_core_credit_account_terms, do:true"`
	Id                      any        //
	TenantId                any        //
	TermsNo                 any        //
	CreditAccountNo         any        //
	CustomerNo              any        //
	CurrencyCode            any        //
	TermsVersion            any        //
	ApplicationNo           any        //
	PolicyNo                any        //
	PolicyVersion           any        //
	TotalLimitMinor         any        //
	BillingDay              any        //
	RepaymentPeriodDays     any        //
	GracePeriodDays         any        //
	DailyLateFeeBasisPoints any        //
	MaximumLateFeeMinor     any        //
	EffectiveFrom           *time.Time //
	EffectiveUntil          *time.Time //
	ApprovedBy              any        //
	ApprovalDigest          any        //
	ContentDigest           any        //
	CreatedAt               *time.Time //
}
