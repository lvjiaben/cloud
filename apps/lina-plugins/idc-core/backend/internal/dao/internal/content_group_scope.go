// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentGroupScopeDao is the data access object for the table plugin_idc_core_content_group_scope.
type ContentGroupScopeDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  ContentGroupScopeColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// ContentGroupScopeColumns defines and stores column names for the table plugin_idc_core_content_group_scope.
type ContentGroupScopeColumns struct {
	Id        string //
	TenantId  string //
	ContentNo string //
	GroupNo   string //
	CreatedAt string //
}

// contentGroupScopeColumns holds the columns for the table plugin_idc_core_content_group_scope.
var contentGroupScopeColumns = ContentGroupScopeColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	ContentNo: "content_no",
	GroupNo:   "group_no",
	CreatedAt: "created_at",
}

// NewContentGroupScopeDao creates and returns a new DAO object for table data access.
func NewContentGroupScopeDao(handlers ...gdb.ModelHandler) *ContentGroupScopeDao {
	return &ContentGroupScopeDao{
		group:    "default",
		table:    "plugin_idc_core_content_group_scope",
		columns:  contentGroupScopeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentGroupScopeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentGroupScopeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentGroupScopeDao) Columns() ContentGroupScopeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentGroupScopeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentGroupScopeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentGroupScopeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
