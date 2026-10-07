// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketSlaReminder is the golang structure for table support_ticket_sla_reminder.
type SupportTicketSlaReminder struct {
	Id          int64      `json:"id"          orm:"id"           description:""`
	TenantId    int64      `json:"tenantId"    orm:"tenant_id"    description:""`
	ReminderNo  string     `json:"reminderNo"  orm:"reminder_no"  description:""`
	TicketNo    string     `json:"ticketNo"    orm:"ticket_no"    description:""`
	RoundNo     int        `json:"roundNo"     orm:"round_no"     description:""`
	TimerCode   string     `json:"timerCode"   orm:"timer_code"   description:""`
	NodeMinutes int        `json:"nodeMinutes" orm:"node_minutes" description:""`
	EventNo     string     `json:"eventNo"     orm:"event_no"     description:""`
	OccurredAt  *time.Time `json:"occurredAt"  orm:"occurred_at"  description:""`
}
