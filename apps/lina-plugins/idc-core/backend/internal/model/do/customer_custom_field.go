// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomField is the golang structure of table plugin_idc_core_customer_custom_field for DAO operations like Where/Data.
type CustomerCustomField struct {
	g.Meta            `orm:"table:plugin_idc_core_customer_custom_field, do:true"`
	Id                any        //
	TenantId          any        //
	FieldNo           any        //
	FieldKey          any        //
	LabelZhCn         any        //
	LabelEnUs         any        //
	HelpZhCn          any        //
	HelpEnUs          any        //
	DataTypeCode      any        //
	CustomerTypeCode  any        //
	VisibilityCode    any        //
	SensitivityCode   any        // Controls masking, export, audit, and AI exclusion policy
	Required          any        //
	CustomerEditable  any        //
	MinimumLength     *int       //
	MaximumLength     *int       //
	MinimumNumber     *string    //
	MaximumNumber     *string    //
	ValidationPattern any        // Bounded service-validated pattern; never evaluated by an unbounded backtracking engine
	Priority          any        //
	StatusCode        any        //
	Version           any        //
	CreatedBy         any        //
	UpdatedBy         any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
