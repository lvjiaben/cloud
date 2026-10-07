// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketEscalation is the golang structure of table plugin_idc_core_support_ticket_escalation for DAO operations like Where/Data.
type SupportTicketEscalation struct {
	g.Meta       `orm:"table:plugin_idc_core_support_ticket_escalation, do:true"`
	Id           any        //
	TenantId     any        //
	EscalationNo any        //
	TicketNo     any        //
	FromUserId   any        //
	ToUserId     any        //
	Reason       any        //
	CreatedAt    *time.Time //
}
