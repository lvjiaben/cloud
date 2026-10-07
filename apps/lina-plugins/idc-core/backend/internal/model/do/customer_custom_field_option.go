// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldOption is the golang structure of table plugin_idc_core_customer_custom_field_option for DAO operations like Where/Data.
type CustomerCustomFieldOption struct {
	g.Meta      `orm:"table:plugin_idc_core_customer_custom_field_option, do:true"`
	Id          any        //
	TenantId    any        //
	OptionNo    any        //
	FieldNo     any        //
	OptionValue any        //
	LabelZhCn   any        //
	LabelEnUs   any        //
	Priority    any        //
	StatusCode  any        //
	CreatedAt   *time.Time //
	UpdatedAt   *time.Time //
}
