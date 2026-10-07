// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketSlaReminderDao is the data access object for the table plugin_idc_core_support_ticket_sla_reminder.
type SupportTicketSlaReminderDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  SupportTicketSlaReminderColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// SupportTicketSlaReminderColumns defines and stores column names for the table plugin_idc_core_support_ticket_sla_reminder.
type SupportTicketSlaReminderColumns struct {
	Id          string //
	TenantId    string //
	ReminderNo  string //
	TicketNo    string //
	RoundNo     string //
	TimerCode   string //
	NodeMinutes string //
	EventNo     string //
	OccurredAt  string //
}

// supportTicketSlaReminderColumns holds the columns for the table plugin_idc_core_support_ticket_sla_reminder.
var supportTicketSlaReminderColumns = SupportTicketSlaReminderColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	ReminderNo:  "reminder_no",
	TicketNo:    "ticket_no",
	RoundNo:     "round_no",
	TimerCode:   "timer_code",
	NodeMinutes: "node_minutes",
	EventNo:     "event_no",
	OccurredAt:  "occurred_at",
}

// NewSupportTicketSlaReminderDao creates and returns a new DAO object for table data access.
func NewSupportTicketSlaReminderDao(handlers ...gdb.ModelHandler) *SupportTicketSlaReminderDao {
	return &SupportTicketSlaReminderDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_sla_reminder",
		columns:  supportTicketSlaReminderColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketSlaReminderDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketSlaReminderDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketSlaReminderDao) Columns() SupportTicketSlaReminderColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketSlaReminderDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketSlaReminderDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketSlaReminderDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
