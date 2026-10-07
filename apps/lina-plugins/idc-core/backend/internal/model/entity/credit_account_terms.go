// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditAccountTerms is the golang structure for table credit_account_terms.
type CreditAccountTerms struct {
	Id                      int64      `json:"id"                      orm:"id"                          description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                   description:""`
	TermsNo                 string     `json:"termsNo"                 orm:"terms_no"                    description:""`
	CreditAccountNo         string     `json:"creditAccountNo"         orm:"credit_account_no"           description:""`
	CustomerNo              string     `json:"customerNo"              orm:"customer_no"                 description:""`
	CurrencyCode            string     `json:"currencyCode"            orm:"currency_code"               description:""`
	TermsVersion            int64      `json:"termsVersion"            orm:"terms_version"               description:""`
	ApplicationNo           string     `json:"applicationNo"           orm:"application_no"              description:""`
	PolicyNo                string     `json:"policyNo"                orm:"policy_no"                   description:""`
	PolicyVersion           int64      `json:"policyVersion"           orm:"policy_version"              description:""`
	TotalLimitMinor         int64      `json:"totalLimitMinor"         orm:"total_limit_minor"           description:""`
	BillingDay              int        `json:"billingDay"              orm:"billing_day"                 description:""`
	RepaymentPeriodDays     int        `json:"repaymentPeriodDays"     orm:"repayment_period_days"       description:""`
	GracePeriodDays         int        `json:"gracePeriodDays"         orm:"grace_period_days"           description:""`
	DailyLateFeeBasisPoints int        `json:"dailyLateFeeBasisPoints" orm:"daily_late_fee_basis_points" description:""`
	MaximumLateFeeMinor     int64      `json:"maximumLateFeeMinor"     orm:"maximum_late_fee_minor"      description:""`
	EffectiveFrom           *time.Time `json:"effectiveFrom"           orm:"effective_from"              description:""`
	EffectiveUntil          *time.Time `json:"effectiveUntil"          orm:"effective_until"             description:""`
	ApprovedBy              int        `json:"approvedBy"              orm:"approved_by"                 description:""`
	ApprovalDigest          string     `json:"approvalDigest"          orm:"approval_digest"             description:""`
	ContentDigest           string     `json:"contentDigest"           orm:"content_digest"              description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                  description:""`
}
