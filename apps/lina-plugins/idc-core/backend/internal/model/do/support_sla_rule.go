// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportSlaRule is the golang structure of table plugin_idc_core_support_sla_rule for DAO operations like Where/Data.
type SupportSlaRule struct {
	g.Meta               `orm:"table:plugin_idc_core_support_sla_rule, do:true"`
	Id                   any        //
	TenantId             any        //
	RuleNo               any        //
	DepartmentNo         any        //
	CategoryNo           any        //
	PriorityCode         any        //
	CalendarNo           any        //
	FirstResponseMinutes any        //
	ResolutionMinutes    any        //
	NearDueMinutes       []int32    //
	StatusCode           any        //
	Version              any        //
	CreatedBy            any        //
	UpdatedBy            any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
