// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketCustomFieldValueOptionDao is the data access object for the table plugin_idc_core_support_ticket_custom_field_value_option.
type SupportTicketCustomFieldValueOptionDao struct {
	table    string                                     // table is the underlying table name of the DAO.
	group    string                                     // group is the database configuration group name of the current DAO.
	columns  SupportTicketCustomFieldValueOptionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                         // handlers for customized model modification.
}

// SupportTicketCustomFieldValueOptionColumns defines and stores column names for the table plugin_idc_core_support_ticket_custom_field_value_option.
type SupportTicketCustomFieldValueOptionColumns struct {
	Id        string //
	TenantId  string //
	TicketNo  string //
	FieldNo   string //
	OptionNo  string //
	NameZhCn  string //
	NameEnUs  string //
	SortOrder string //
	CreatedAt string //
}

// supportTicketCustomFieldValueOptionColumns holds the columns for the table plugin_idc_core_support_ticket_custom_field_value_option.
var supportTicketCustomFieldValueOptionColumns = SupportTicketCustomFieldValueOptionColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	TicketNo:  "ticket_no",
	FieldNo:   "field_no",
	OptionNo:  "option_no",
	NameZhCn:  "name_zh_cn",
	NameEnUs:  "name_en_us",
	SortOrder: "sort_order",
	CreatedAt: "created_at",
}

// NewSupportTicketCustomFieldValueOptionDao creates and returns a new DAO object for table data access.
func NewSupportTicketCustomFieldValueOptionDao(handlers ...gdb.ModelHandler) *SupportTicketCustomFieldValueOptionDao {
	return &SupportTicketCustomFieldValueOptionDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_custom_field_value_option",
		columns:  supportTicketCustomFieldValueOptionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketCustomFieldValueOptionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketCustomFieldValueOptionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketCustomFieldValueOptionDao) Columns() SupportTicketCustomFieldValueOptionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketCustomFieldValueOptionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketCustomFieldValueOptionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketCustomFieldValueOptionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
