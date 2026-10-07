// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportDepartment is the golang structure of table plugin_idc_core_support_department for DAO operations like Where/Data.
type SupportDepartment struct {
	g.Meta             `orm:"table:plugin_idc_core_support_department, do:true"`
	Id                 any        //
	TenantId           any        //
	DepartmentNo       any        //
	NameZhCn           any        //
	NameEnUs           any        //
	ParentDepartmentNo any        //
	AssignmentMode     any        //
	RoundRobinCursor   any        //
	StatusCode         any        //
	Version            any        //
	CreatedBy          any        //
	UpdatedBy          any        //
	CreatedAt          *time.Time //
	UpdatedAt          *time.Time //
}
