// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldValueOption is the golang structure of table plugin_idc_core_customer_custom_field_value_option for DAO operations like Where/Data.
type CustomerCustomFieldValueOption struct {
	g.Meta     `orm:"table:plugin_idc_core_customer_custom_field_value_option, do:true"`
	Id         any        //
	TenantId   any        //
	CustomerNo any        //
	FieldNo    any        //
	ValueNo    any        //
	OptionNo   any        //
	CreatedAt  *time.Time //
}
