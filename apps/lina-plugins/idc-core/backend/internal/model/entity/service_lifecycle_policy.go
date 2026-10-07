// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceLifecyclePolicy is the golang structure for table service_lifecycle_policy.
type ServiceLifecyclePolicy struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	PolicyNo              string     `json:"policyNo"              orm:"policy_no"               description:""`
	ServiceNo             string     `json:"serviceNo"             orm:"service_no"              description:""`
	PolicyVersion         int64      `json:"policyVersion"         orm:"policy_version"          description:""`
	ReminderCount         int        `json:"reminderCount"         orm:"reminder_count"          description:""`
	GraceSeconds          int64      `json:"graceSeconds"          orm:"grace_seconds"           description:""`
	SuspendEnabled        bool       `json:"suspendEnabled"        orm:"suspend_enabled"         description:""`
	SuspendAfterSeconds   *int64     `json:"suspendAfterSeconds"   orm:"suspend_after_seconds"   description:""`
	TerminateEnabled      bool       `json:"terminateEnabled"      orm:"terminate_enabled"       description:""`
	TerminateAfterSeconds *int64     `json:"terminateAfterSeconds" orm:"terminate_after_seconds" description:""`
	ConfiguredBy          int        `json:"configuredBy"          orm:"configured_by"           description:""`
	ContentDigest         string     `json:"contentDigest"         orm:"content_digest"          description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
}
