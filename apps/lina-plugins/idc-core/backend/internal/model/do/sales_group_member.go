// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesGroupMember is the golang structure of table plugin_idc_core_sales_group_member for DAO operations like Where/Data.
type SalesGroupMember struct {
	g.Meta       `orm:"table:plugin_idc_core_sales_group_member, do:true"`
	Id           any        //
	TenantId     any        //
	MembershipNo any        //
	GroupNo      any        //
	StaffUserId  any        //
	RoleCode     any        //
	StatusCode   any        //
	EffectiveAt  *time.Time //
	EndedAt      *time.Time //
	CreatedAt    *time.Time //
}
