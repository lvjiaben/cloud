// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketSlaRoundDao is the data access object for the table plugin_idc_core_support_ticket_sla_round.
type SupportTicketSlaRoundDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SupportTicketSlaRoundColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SupportTicketSlaRoundColumns defines and stores column names for the table plugin_idc_core_support_ticket_sla_round.
type SupportTicketSlaRoundColumns struct {
	Id                    string //
	TenantId              string //
	TicketNo              string //
	RoundNo               string //
	RuleNo                string //
	RuleVersion           string //
	CalendarNo            string //
	CalendarVersion       string //
	CalendarMode          string //
	TimezoneName          string //
	Workdays              string //
	WorkStartMinute       string //
	WorkEndMinute         string //
	HolidayDates          string //
	FirstResponseMinutes  string //
	ResolutionMinutes     string //
	NearDueMinutes        string //
	FirstResponseDueAt    string //
	ResolutionDueAt       string //
	FirstRespondedAt      string //
	ResolvedAt            string //
	StoppedAt             string //
	PausedSeconds         string //
	FirstResponseBreached string //
	ResolutionBreached    string //
	CreatedAt             string //
}

// supportTicketSlaRoundColumns holds the columns for the table plugin_idc_core_support_ticket_sla_round.
var supportTicketSlaRoundColumns = SupportTicketSlaRoundColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	TicketNo:              "ticket_no",
	RoundNo:               "round_no",
	RuleNo:                "rule_no",
	RuleVersion:           "rule_version",
	CalendarNo:            "calendar_no",
	CalendarVersion:       "calendar_version",
	CalendarMode:          "calendar_mode",
	TimezoneName:          "timezone_name",
	Workdays:              "workdays",
	WorkStartMinute:       "work_start_minute",
	WorkEndMinute:         "work_end_minute",
	HolidayDates:          "holiday_dates",
	FirstResponseMinutes:  "first_response_minutes",
	ResolutionMinutes:     "resolution_minutes",
	NearDueMinutes:        "near_due_minutes",
	FirstResponseDueAt:    "first_response_due_at",
	ResolutionDueAt:       "resolution_due_at",
	FirstRespondedAt:      "first_responded_at",
	ResolvedAt:            "resolved_at",
	StoppedAt:             "stopped_at",
	PausedSeconds:         "paused_seconds",
	FirstResponseBreached: "first_response_breached",
	ResolutionBreached:    "resolution_breached",
	CreatedAt:             "created_at",
}

// NewSupportTicketSlaRoundDao creates and returns a new DAO object for table data access.
func NewSupportTicketSlaRoundDao(handlers ...gdb.ModelHandler) *SupportTicketSlaRoundDao {
	return &SupportTicketSlaRoundDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_sla_round",
		columns:  supportTicketSlaRoundColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketSlaRoundDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketSlaRoundDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketSlaRoundDao) Columns() SupportTicketSlaRoundColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketSlaRoundDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketSlaRoundDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *SupportTicketSlaRoundDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
