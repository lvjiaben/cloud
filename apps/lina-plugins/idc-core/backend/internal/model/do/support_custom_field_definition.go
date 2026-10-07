// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportCustomFieldDefinition is the golang structure of table plugin_idc_core_support_custom_field_definition for DAO operations like Where/Data.
type SupportCustomFieldDefinition struct {
	g.Meta          `orm:"table:plugin_idc_core_support_custom_field_definition, do:true"`
	Id              any        //
	TenantId        any        //
	FieldNo         any        //
	FieldType       any        //
	NameZhCn        any        //
	NameEnUs        any        //
	DescriptionZhCn any        //
	DescriptionEnUs any        //
	AppliesPresales any        //
	AppliesCustomer any        //
	AppliesInternal any        //
	VisibilityCode  any        //
	Required        any        //
	SortOrder       any        //
	StatusCode      any        //
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
