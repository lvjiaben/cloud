// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketSlaRound is the golang structure of table plugin_idc_core_support_ticket_sla_round for DAO operations like Where/Data.
type SupportTicketSlaRound struct {
	g.Meta                `orm:"table:plugin_idc_core_support_ticket_sla_round, do:true"`
	Id                    any        //
	TenantId              any        //
	TicketNo              any        //
	RoundNo               any        //
	RuleNo                any        //
	RuleVersion           any        //
	CalendarNo            any        //
	CalendarVersion       any        //
	CalendarMode          any        //
	TimezoneName          any        //
	Workdays              []int32    //
	WorkStartMinute       any        //
	WorkEndMinute         any        //
	HolidayDates          any        //
	FirstResponseMinutes  any        //
	ResolutionMinutes     any        //
	NearDueMinutes        []int32    //
	FirstResponseDueAt    *time.Time //
	ResolutionDueAt       *time.Time //
	FirstRespondedAt      *time.Time //
	ResolvedAt            *time.Time //
	StoppedAt             *time.Time //
	PausedSeconds         any        //
	FirstResponseBreached any        //
	ResolutionBreached    any        //
	CreatedAt             *time.Time //
}
