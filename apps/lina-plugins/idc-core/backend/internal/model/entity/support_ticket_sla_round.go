// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketSlaRound is the golang structure for table support_ticket_sla_round.
type SupportTicketSlaRound struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int64      `json:"tenantId"              orm:"tenant_id"               description:""`
	TicketNo              string     `json:"ticketNo"              orm:"ticket_no"               description:""`
	RoundNo               int        `json:"roundNo"               orm:"round_no"                description:""`
	RuleNo                string     `json:"ruleNo"                orm:"rule_no"                 description:""`
	RuleVersion           int64      `json:"ruleVersion"           orm:"rule_version"            description:""`
	CalendarNo            string     `json:"calendarNo"            orm:"calendar_no"             description:""`
	CalendarVersion       int64      `json:"calendarVersion"       orm:"calendar_version"        description:""`
	CalendarMode          string     `json:"calendarMode"          orm:"calendar_mode"           description:""`
	TimezoneName          string     `json:"timezoneName"          orm:"timezone_name"           description:""`
	Workdays              []int32    `json:"workdays"              orm:"workdays"                description:""`
	WorkStartMinute       int        `json:"workStartMinute"       orm:"work_start_minute"       description:""`
	WorkEndMinute         int        `json:"workEndMinute"         orm:"work_end_minute"         description:""`
	HolidayDates          time.Time  `json:"holidayDates"          orm:"holiday_dates"           description:""`
	FirstResponseMinutes  int        `json:"firstResponseMinutes"  orm:"first_response_minutes"  description:""`
	ResolutionMinutes     int        `json:"resolutionMinutes"     orm:"resolution_minutes"      description:""`
	NearDueMinutes        []int32    `json:"nearDueMinutes"        orm:"near_due_minutes"        description:""`
	FirstResponseDueAt    *time.Time `json:"firstResponseDueAt"    orm:"first_response_due_at"   description:""`
	ResolutionDueAt       *time.Time `json:"resolutionDueAt"       orm:"resolution_due_at"       description:""`
	FirstRespondedAt      *time.Time `json:"firstRespondedAt"      orm:"first_responded_at"      description:""`
	ResolvedAt            *time.Time `json:"resolvedAt"            orm:"resolved_at"             description:""`
	StoppedAt             *time.Time `json:"stoppedAt"             orm:"stopped_at"              description:""`
	PausedSeconds         int64      `json:"pausedSeconds"         orm:"paused_seconds"          description:""`
	FirstResponseBreached bool       `json:"firstResponseBreached" orm:"first_response_breached" description:""`
	ResolutionBreached    bool       `json:"resolutionBreached"    orm:"resolution_breached"     description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
}
