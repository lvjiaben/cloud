// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Tenant is the golang structure of table plugin_tenant_core_tenant for DAO operations like Where/Data.
type Tenant struct {
	g.Meta    `orm:"table:plugin_tenant_core_tenant, do:true"`
	Id        any        //
	Code      any        //
	Name      any        //
	Status    any        //
	Remark    any        //
	CreatedBy any        //
	UpdatedBy any        //
	CreatedAt *time.Time //
	UpdatedAt *time.Time //
	DeletedAt *time.Time //
}
