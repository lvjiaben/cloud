// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketCustomFieldValue is the golang structure of table plugin_idc_core_support_ticket_custom_field_value for DAO operations like Where/Data.
type SupportTicketCustomFieldValue struct {
	g.Meta              `orm:"table:plugin_idc_core_support_ticket_custom_field_value, do:true"`
	Id                  any        //
	TenantId            any        //
	TicketNo            any        //
	FieldNo             any        //
	DefinitionVersion   any        //
	FieldType           any        //
	NameZhCn            any        //
	NameEnUs            any        //
	DescriptionZhCn     any        //
	DescriptionEnUs     any        //
	VisibilityCode      any        //
	Required            any        //
	SortOrder           any        //
	NormalizedValue     any        //
	SelectedOptionCount any        //
	CreatedAt           *time.Time //
}
