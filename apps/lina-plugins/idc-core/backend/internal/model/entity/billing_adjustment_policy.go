// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingAdjustmentPolicy is the golang structure for table billing_adjustment_policy.
type BillingAdjustmentPolicy struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	PolicyNo             string     `json:"policyNo"             orm:"policy_no"              description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"          description:""`
	ReviewThresholdMinor int64      `json:"reviewThresholdMinor" orm:"review_threshold_minor" description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	ConfiguredBy         int        `json:"configuredBy"         orm:"configured_by"          description:""`
	ContentDigest        string     `json:"contentDigest"        orm:"content_digest"         description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
}
