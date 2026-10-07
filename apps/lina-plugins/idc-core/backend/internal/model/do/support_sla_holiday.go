// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportSlaHoliday is the golang structure of table plugin_idc_core_support_sla_holiday for DAO operations like Where/Data.
type SupportSlaHoliday struct {
	g.Meta      `orm:"table:plugin_idc_core_support_sla_holiday, do:true"`
	Id          any        //
	TenantId    any        //
	CalendarNo  any        //
	HolidayDate any        //
	Name        any        //
	CreatedAt   *time.Time //
}
