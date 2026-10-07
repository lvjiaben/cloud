// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketUpstreamItemDao is the data access object for the table plugin_idc_core_support_ticket_upstream_item.
type SupportTicketUpstreamItemDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  SupportTicketUpstreamItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// SupportTicketUpstreamItemColumns defines and stores column names for the table plugin_idc_core_support_ticket_upstream_item.
type SupportTicketUpstreamItemColumns struct {
	Id        string //
	TenantId  string //
	ForwardNo string //
	ItemType  string //
	ItemNo    string //
	SortOrder string //
	CreatedAt string //
}

// supportTicketUpstreamItemColumns holds the columns for the table plugin_idc_core_support_ticket_upstream_item.
var supportTicketUpstreamItemColumns = SupportTicketUpstreamItemColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	ForwardNo: "forward_no",
	ItemType:  "item_type",
	ItemNo:    "item_no",
	SortOrder: "sort_order",
	CreatedAt: "created_at",
}

// NewSupportTicketUpstreamItemDao creates and returns a new DAO object for table data access.
func NewSupportTicketUpstreamItemDao(handlers ...gdb.ModelHandler) *SupportTicketUpstreamItemDao {
	return &SupportTicketUpstreamItemDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_upstream_item",
		columns:  supportTicketUpstreamItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketUpstreamItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketUpstreamItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketUpstreamItemDao) Columns() SupportTicketUpstreamItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketUpstreamItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketUpstreamItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketUpstreamItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
