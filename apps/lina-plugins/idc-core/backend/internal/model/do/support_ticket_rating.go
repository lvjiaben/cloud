// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketRating is the golang structure of table plugin_idc_core_support_ticket_rating for DAO operations like Where/Data.
type SupportTicketRating struct {
	g.Meta     `orm:"table:plugin_idc_core_support_ticket_rating, do:true"`
	Id         any        //
	TenantId   any        //
	RatingNo   any        //
	TicketNo   any        //
	RoundNo    any        //
	Score      any        //
	Comment    any        //
	AuthorType any        //
	CreatedAt  *time.Time //
}
