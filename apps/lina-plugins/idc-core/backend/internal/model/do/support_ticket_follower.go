// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketFollower is the golang structure of table plugin_idc_core_support_ticket_follower for DAO operations like Where/Data.
type SupportTicketFollower struct {
	g.Meta    `orm:"table:plugin_idc_core_support_ticket_follower, do:true"`
	Id        any        //
	TenantId  any        //
	TicketNo  any        //
	UserId    any        //
	CreatedBy any        //
	CreatedAt *time.Time //
}
