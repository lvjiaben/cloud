// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketStatusHistoryDao is the data access object for the table plugin_idc_core_support_ticket_status_history.
type SupportTicketStatusHistoryDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  SupportTicketStatusHistoryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// SupportTicketStatusHistoryColumns defines and stores column names for the table plugin_idc_core_support_ticket_status_history.
type SupportTicketStatusHistoryColumns struct {
	Id             string //
	TenantId       string //
	HistoryNo      string //
	TicketNo       string //
	FromBaseStage  string //
	ToBaseStage    string //
	CustomStatusNo string //
	SourceCode     string //
	ActorUserId    string //
	ReasonCode     string //
	ReasonText     string //
	RuleVersion    string //
	OccurredAt     string //
}

// supportTicketStatusHistoryColumns holds the columns for the table plugin_idc_core_support_ticket_status_history.
var supportTicketStatusHistoryColumns = SupportTicketStatusHistoryColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	HistoryNo:      "history_no",
	TicketNo:       "ticket_no",
	FromBaseStage:  "from_base_stage",
	ToBaseStage:    "to_base_stage",
	CustomStatusNo: "custom_status_no",
	SourceCode:     "source_code",
	ActorUserId:    "actor_user_id",
	ReasonCode:     "reason_code",
	ReasonText:     "reason_text",
	RuleVersion:    "rule_version",
	OccurredAt:     "occurred_at",
}

// NewSupportTicketStatusHistoryDao creates and returns a new DAO object for table data access.
func NewSupportTicketStatusHistoryDao(handlers ...gdb.ModelHandler) *SupportTicketStatusHistoryDao {
	return &SupportTicketStatusHistoryDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_status_history",
		columns:  supportTicketStatusHistoryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketStatusHistoryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketStatusHistoryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketStatusHistoryDao) Columns() SupportTicketStatusHistoryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketStatusHistoryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketStatusHistoryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketStatusHistoryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
