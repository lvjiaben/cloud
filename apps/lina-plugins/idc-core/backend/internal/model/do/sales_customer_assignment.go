// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCustomerAssignment is the golang structure of table plugin_idc_core_sales_customer_assignment for DAO operations like Where/Data.
type SalesCustomerAssignment struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_customer_assignment, do:true"`
	Id                  any        //
	TenantId            any        //
	AssignmentNo        any        //
	CustomerNo          any        //
	MembershipNo        any        //
	ProtectionExpiresAt *time.Time //
	EffectiveAt         *time.Time //
	EndedAt             *time.Time //
	StatusCode          any        //
	SourceCode          any        //
	CreatedAt           *time.Time //
}
