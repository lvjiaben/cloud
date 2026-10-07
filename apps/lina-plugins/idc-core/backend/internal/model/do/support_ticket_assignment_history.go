// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAssignmentHistory is the golang structure of table plugin_idc_core_support_ticket_assignment_history for DAO operations like Where/Data.
type SupportTicketAssignmentHistory struct {
	g.Meta           `orm:"table:plugin_idc_core_support_ticket_assignment_history, do:true"`
	Id               any        //
	TenantId         any        //
	HistoryNo        any        //
	TicketNo         any        //
	ActionCode       any        //
	FromDepartmentNo any        //
	ToDepartmentNo   any        //
	FromUserId       any        //
	ToUserId         any        //
	ActorUserId      any        //
	ReasonCode       any        //
	ReasonText       any        //
	OccurredAt       *time.Time //
}
