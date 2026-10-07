// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLevelAssignment is the golang structure of table plugin_idc_core_customer_level_assignment for DAO operations like Where/Data.
type CustomerLevelAssignment struct {
	g.Meta     `orm:"table:plugin_idc_core_customer_level_assignment, do:true"`
	Id         any        //
	TenantId   any        //
	CustomerNo any        //
	LevelNo    any        //
	SourceCode any        //
	Reason     any        //
	StatusCode any        //
	AssignedBy any        //
	AssignedAt *time.Time //
	RevokedBy  any        //
	RevokedAt  *time.Time //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
