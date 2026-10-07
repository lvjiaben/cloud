// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketSlaReminder is the golang structure of table plugin_idc_core_support_ticket_sla_reminder for DAO operations like Where/Data.
type SupportTicketSlaReminder struct {
	g.Meta      `orm:"table:plugin_idc_core_support_ticket_sla_reminder, do:true"`
	Id          any        //
	TenantId    any        //
	ReminderNo  any        //
	TicketNo    any        //
	RoundNo     any        //
	TimerCode   any        //
	NodeMinutes any        //
	EventNo     any        //
	OccurredAt  *time.Time //
}
