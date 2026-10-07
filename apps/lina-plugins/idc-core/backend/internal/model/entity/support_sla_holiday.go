// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportSlaHoliday is the golang structure for table support_sla_holiday.
type SupportSlaHoliday struct {
	Id          int64      `json:"id"          orm:"id"           description:""`
	TenantId    int64      `json:"tenantId"    orm:"tenant_id"    description:""`
	CalendarNo  string     `json:"calendarNo"  orm:"calendar_no"  description:""`
	HolidayDate time.Time  `json:"holidayDate" orm:"holiday_date" description:""`
	Name        string     `json:"name"        orm:"name"         description:""`
	CreatedAt   *time.Time `json:"createdAt"   orm:"created_at"   description:""`
}
