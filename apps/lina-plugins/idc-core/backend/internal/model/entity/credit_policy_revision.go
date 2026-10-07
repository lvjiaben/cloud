// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditPolicyRevision is the golang structure for table credit_policy_revision.
type CreditPolicyRevision struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	PolicyNo                 string     `json:"policyNo"                 orm:"policy_no"                   description:""`
	PolicyVersion            int64      `json:"policyVersion"            orm:"policy_version"              description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"               description:""`
	MaximumLimitMinor        int64      `json:"maximumLimitMinor"        orm:"maximum_limit_minor"         description:""`
	DualReviewThresholdMinor int64      `json:"dualReviewThresholdMinor" orm:"dual_review_threshold_minor" description:""`
	BillingDay               int        `json:"billingDay"               orm:"billing_day"                 description:""`
	RepaymentPeriodDays      int        `json:"repaymentPeriodDays"      orm:"repayment_period_days"       description:""`
	GracePeriodDays          int        `json:"gracePeriodDays"          orm:"grace_period_days"           description:""`
	DailyLateFeeBasisPoints  int        `json:"dailyLateFeeBasisPoints"  orm:"daily_late_fee_basis_points" description:""`
	MaximumLateFeeMinor      int64      `json:"maximumLateFeeMinor"      orm:"maximum_late_fee_minor"      description:""`
	ConfiguredBy             int        `json:"configuredBy"             orm:"configured_by"               description:""`
	ContentDigest            string     `json:"contentDigest"            orm:"content_digest"              description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
}
