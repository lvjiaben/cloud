// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportAutomationPolicy is the golang structure for table support_automation_policy.
type SupportAutomationPolicy struct {
	Id            int64      `json:"id"            orm:"id"              description:""`
	TenantId      int64      `json:"tenantId"      orm:"tenant_id"       description:""`
	PolicyNo      string     `json:"policyNo"      orm:"policy_no"       description:""`
	AutoCloseDays int        `json:"autoCloseDays" orm:"auto_close_days" description:""`
	ReopenDays    int        `json:"reopenDays"    orm:"reopen_days"     description:""`
	Version       int64      `json:"version"       orm:"version"         description:""`
	CreatedBy     int64      `json:"createdBy"     orm:"created_by"      description:""`
	UpdatedBy     int64      `json:"updatedBy"     orm:"updated_by"      description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"      description:""`
	UpdatedAt     *time.Time `json:"updatedAt"     orm:"updated_at"      description:""`
}
