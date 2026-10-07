// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceRenewalPolicy is the golang structure for table service_renewal_policy.
type ServiceRenewalPolicy struct {
	Id                         int64      `json:"id"                         orm:"id"                           description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	PolicyNo                   string     `json:"policyNo"                   orm:"policy_no"                    description:""`
	EntitlementNo              string     `json:"entitlementNo"              orm:"entitlement_no"               description:""`
	PolicyVersion              int64      `json:"policyVersion"              orm:"policy_version"               description:""`
	AutoRenewEnabled           bool       `json:"autoRenewEnabled"           orm:"auto_renew_enabled"           description:""`
	RenewalCycles              int        `json:"renewalCycles"              orm:"renewal_cycles"               description:""`
	LeadSeconds                int64      `json:"leadSeconds"                orm:"lead_seconds"                 description:""`
	MaximumAttempts            int        `json:"maximumAttempts"            orm:"maximum_attempts"             description:""`
	RetryIntervalSeconds       int64      `json:"retryIntervalSeconds"       orm:"retry_interval_seconds"       description:""`
	FailureNotificationEnabled bool       `json:"failureNotificationEnabled" orm:"failure_notification_enabled" description:""`
	ConfiguredBy               int        `json:"configuredBy"               orm:"configured_by"                description:""`
	ContentDigest              string     `json:"contentDigest"              orm:"content_digest"               description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
}
