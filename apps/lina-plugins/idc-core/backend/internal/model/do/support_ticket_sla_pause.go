// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketSlaPause is the golang structure of table plugin_idc_core_support_ticket_sla_pause for DAO operations like Where/Data.
type SupportTicketSlaPause struct {
	g.Meta     `orm:"table:plugin_idc_core_support_ticket_sla_pause, do:true"`
	Id         any        //
	TenantId   any        //
	TicketNo   any        //
	RoundNo    any        //
	StartedAt  *time.Time //
	EndedAt    *time.Time //
	ReasonCode any        //
}
