// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportSlaCalendar is the golang structure of table plugin_idc_core_support_sla_calendar for DAO operations like Where/Data.
type SupportSlaCalendar struct {
	g.Meta          `orm:"table:plugin_idc_core_support_sla_calendar, do:true"`
	Id              any        //
	TenantId        any        //
	CalendarNo      any        //
	Name            any        //
	ModeCode        any        //
	TimezoneName    any        //
	Workdays        []int32    //
	WorkStartMinute any        //
	WorkEndMinute   any        //
	StatusCode      any        //
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
