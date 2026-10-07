// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceLifecycleReminderRule is the golang structure for table service_lifecycle_reminder_rule.
type ServiceLifecycleReminderRule struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"         description:""`
	ReminderRuleNo   string     `json:"reminderRuleNo"   orm:"reminder_rule_no"  description:""`
	PolicyNo         string     `json:"policyNo"         orm:"policy_no"         description:""`
	PolicyVersion    int64      `json:"policyVersion"    orm:"policy_version"    description:""`
	ServiceNo        string     `json:"serviceNo"        orm:"service_no"        description:""`
	ReminderSequence int        `json:"reminderSequence" orm:"reminder_sequence" description:""`
	RelativeSeconds  int64      `json:"relativeSeconds"  orm:"relative_seconds"  description:""`
	ContentDigest    string     `json:"contentDigest"    orm:"content_digest"    description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
}
