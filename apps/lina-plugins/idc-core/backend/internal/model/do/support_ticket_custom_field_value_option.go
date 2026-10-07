// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketCustomFieldValueOption is the golang structure of table plugin_idc_core_support_ticket_custom_field_value_option for DAO operations like Where/Data.
type SupportTicketCustomFieldValueOption struct {
	g.Meta    `orm:"table:plugin_idc_core_support_ticket_custom_field_value_option, do:true"`
	Id        any        //
	TenantId  any        //
	TicketNo  any        //
	FieldNo   any        //
	OptionNo  any        //
	NameZhCn  any        //
	NameEnUs  any        //
	SortOrder any        //
	CreatedAt *time.Time //
}
