// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportCustomFieldOption is the golang structure of table plugin_idc_core_support_custom_field_option for DAO operations like Where/Data.
type SupportCustomFieldOption struct {
	g.Meta     `orm:"table:plugin_idc_core_support_custom_field_option, do:true"`
	Id         any        //
	TenantId   any        //
	OptionNo   any        //
	FieldNo    any        //
	NameZhCn   any        //
	NameEnUs   any        //
	SortOrder  any        //
	StatusCode any        //
	Version    any        //
	CreatedBy  any        //
	UpdatedBy  any        //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
