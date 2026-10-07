// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketUpstreamItem is the golang structure of table plugin_idc_core_support_ticket_upstream_item for DAO operations like Where/Data.
type SupportTicketUpstreamItem struct {
	g.Meta    `orm:"table:plugin_idc_core_support_ticket_upstream_item, do:true"`
	Id        any        //
	TenantId  any        //
	ForwardNo any        //
	ItemType  any        //
	ItemNo    any        //
	SortOrder any        //
	CreatedAt *time.Time //
}
