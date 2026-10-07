// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketMessage is the golang structure of table plugin_idc_core_support_ticket_message for DAO operations like Where/Data.
type SupportTicketMessage struct {
	g.Meta         `orm:"table:plugin_idc_core_support_ticket_message, do:true"`
	Id             any        //
	TenantId       any        //
	MessageNo      any        //
	TicketNo       any        //
	MessageType    any        //
	VisibilityCode any        //
	AuthorType     any        //
	AuthorUserId   any        //
	Body           any        //
	Automated      any        //
	RuleVersion    any        //
	HiddenAt       *time.Time //
	HiddenBy       any        //
	HiddenReason   any        //
	CreatedAt      *time.Time //
}
