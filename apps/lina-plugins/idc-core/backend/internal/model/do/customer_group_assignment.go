// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerGroupAssignment is the golang structure of table plugin_idc_core_customer_group_assignment for DAO operations like Where/Data.
type CustomerGroupAssignment struct {
	g.Meta     `orm:"table:plugin_idc_core_customer_group_assignment, do:true"`
	Id         any        //
	TenantId   any        //
	CustomerNo any        //
	GroupNo    any        //
	SourceCode any        //
	StatusCode any        //
	AssignedBy any        //
	AssignedAt *time.Time //
	RevokedBy  any        //
	RevokedAt  *time.Time //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
