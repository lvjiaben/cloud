// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCustomerCollaborator is the golang structure of table plugin_idc_core_sales_customer_collaborator for DAO operations like Where/Data.
type SalesCustomerCollaborator struct {
	g.Meta          `orm:"table:plugin_idc_core_sales_customer_collaborator, do:true"`
	Id              any        //
	TenantId        any        //
	CollaborationNo any        //
	AssignmentNo    any        //
	MembershipNo    any        //
	StatusCode      any        //
	EffectiveAt     *time.Time //
	EndedAt         *time.Time //
	CreatedAt       *time.Time //
}
