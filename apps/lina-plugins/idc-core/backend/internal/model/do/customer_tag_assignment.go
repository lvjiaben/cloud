// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerTagAssignment is the golang structure of table plugin_idc_core_customer_tag_assignment for DAO operations like Where/Data.
type CustomerTagAssignment struct {
	g.Meta     `orm:"table:plugin_idc_core_customer_tag_assignment, do:true"`
	Id         any        //
	TenantId   any        //
	CustomerNo any        //
	TagNo      any        //
	SourceCode any        //
	StatusCode any        //
	AssignedBy any        //
	AssignedAt *time.Time //
	RevokedBy  any        //
	RevokedAt  *time.Time //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
