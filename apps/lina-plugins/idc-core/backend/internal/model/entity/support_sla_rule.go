// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportSlaRule is the golang structure for table support_sla_rule.
type SupportSlaRule struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int64      `json:"tenantId"             orm:"tenant_id"              description:""`
	RuleNo               string     `json:"ruleNo"               orm:"rule_no"                description:""`
	DepartmentNo         string     `json:"departmentNo"         orm:"department_no"          description:""`
	CategoryNo           string     `json:"categoryNo"           orm:"category_no"            description:""`
	PriorityCode         string     `json:"priorityCode"         orm:"priority_code"          description:""`
	CalendarNo           string     `json:"calendarNo"           orm:"calendar_no"            description:""`
	FirstResponseMinutes int        `json:"firstResponseMinutes" orm:"first_response_minutes" description:""`
	ResolutionMinutes    int        `json:"resolutionMinutes"    orm:"resolution_minutes"     description:""`
	NearDueMinutes       []int32    `json:"nearDueMinutes"       orm:"near_due_minutes"       description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedBy            int64      `json:"createdBy"            orm:"created_by"             description:""`
	UpdatedBy            int64      `json:"updatedBy"            orm:"updated_by"             description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
