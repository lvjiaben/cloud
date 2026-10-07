// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketStatusHistory is the golang structure of table plugin_idc_core_support_ticket_status_history for DAO operations like Where/Data.
type SupportTicketStatusHistory struct {
	g.Meta         `orm:"table:plugin_idc_core_support_ticket_status_history, do:true"`
	Id             any        //
	TenantId       any        //
	HistoryNo      any        //
	TicketNo       any        //
	FromBaseStage  any        //
	ToBaseStage    any        //
	CustomStatusNo any        //
	SourceCode     any        //
	ActorUserId    any        //
	ReasonCode     any        //
	ReasonText     any        //
	RuleVersion    any        //
	OccurredAt     *time.Time //
}
