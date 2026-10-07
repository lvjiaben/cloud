// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerContactRole is the golang structure of table plugin_idc_core_customer_contact_role for DAO operations like Where/Data.
type CustomerContactRole struct {
	g.Meta     `orm:"table:plugin_idc_core_customer_contact_role, do:true"`
	Id         any        //
	TenantId   any        //
	CustomerNo any        //
	ContactNo  any        //
	RoleCode   any        //
	StatusCode any        //
	AssignedBy any        //
	AssignedAt *time.Time //
	RevokedBy  any        //
	RevokedAt  *time.Time //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
