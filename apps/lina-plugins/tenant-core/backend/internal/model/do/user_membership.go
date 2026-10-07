// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// UserMembership is the golang structure of table plugin_tenant_core_user_membership for DAO operations like Where/Data.
type UserMembership struct {
	g.Meta    `orm:"table:plugin_tenant_core_user_membership, do:true"`
	Id        any        //
	UserId    any        //
	TenantId  any        //
	Status    any        //
	JoinedAt  *time.Time //
	CreatedBy any        //
	UpdatedBy any        //
	CreatedAt *time.Time //
	UpdatedAt *time.Time //
	DeletedAt *time.Time //
}
