// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketMessageDao is the data access object for the table plugin_idc_core_support_ticket_message.
type SupportTicketMessageDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  SupportTicketMessageColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// SupportTicketMessageColumns defines and stores column names for the table plugin_idc_core_support_ticket_message.
type SupportTicketMessageColumns struct {
	Id             string //
	TenantId       string //
	MessageNo      string //
	TicketNo       string //
	MessageType    string //
	VisibilityCode string //
	AuthorType     string //
	AuthorUserId   string //
	Body           string //
	Automated      string //
	RuleVersion    string //
	HiddenAt       string //
	HiddenBy       string //
	HiddenReason   string //
	CreatedAt      string //
}

// supportTicketMessageColumns holds the columns for the table plugin_idc_core_support_ticket_message.
var supportTicketMessageColumns = SupportTicketMessageColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	MessageNo:      "message_no",
	TicketNo:       "ticket_no",
	MessageType:    "message_type",
	VisibilityCode: "visibility_code",
	AuthorType:     "author_type",
	AuthorUserId:   "author_user_id",
	Body:           "body",
	Automated:      "automated",
	RuleVersion:    "rule_version",
	HiddenAt:       "hidden_at",
	HiddenBy:       "hidden_by",
	HiddenReason:   "hidden_reason",
	CreatedAt:      "created_at",
}

// NewSupportTicketMessageDao creates and returns a new DAO object for table data access.
func NewSupportTicketMessageDao(handlers ...gdb.ModelHandler) *SupportTicketMessageDao {
	return &SupportTicketMessageDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_message",
		columns:  supportTicketMessageColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketMessageDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketMessageDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketMessageDao) Columns() SupportTicketMessageColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketMessageDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketMessageDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketMessageDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
