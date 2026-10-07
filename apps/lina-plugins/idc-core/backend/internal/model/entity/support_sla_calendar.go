// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportSlaCalendar is the golang structure for table support_sla_calendar.
type SupportSlaCalendar struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int64      `json:"tenantId"        orm:"tenant_id"         description:""`
	CalendarNo      string     `json:"calendarNo"      orm:"calendar_no"       description:""`
	Name            string     `json:"name"            orm:"name"              description:""`
	ModeCode        string     `json:"modeCode"        orm:"mode_code"         description:""`
	TimezoneName    string     `json:"timezoneName"    orm:"timezone_name"     description:""`
	Workdays        []int32    `json:"workdays"        orm:"workdays"          description:""`
	WorkStartMinute int        `json:"workStartMinute" orm:"work_start_minute" description:""`
	WorkEndMinute   int        `json:"workEndMinute"   orm:"work_end_minute"   description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	Version         int64      `json:"version"         orm:"version"           description:""`
	CreatedBy       int64      `json:"createdBy"       orm:"created_by"        description:""`
	UpdatedBy       int64      `json:"updatedBy"       orm:"updated_by"        description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
