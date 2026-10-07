// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportDepartmentMember is the golang structure of table plugin_idc_core_support_department_member for DAO operations like Where/Data.
type SupportDepartmentMember struct {
	g.Meta       `orm:"table:plugin_idc_core_support_department_member, do:true"`
	Id           any        //
	TenantId     any        //
	DepartmentNo any        //
	UserId       any        //
	RoleCode     any        //
	StatusCode   any        //
	Version      any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
