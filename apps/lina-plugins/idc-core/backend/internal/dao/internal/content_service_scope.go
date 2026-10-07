// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentServiceScopeDao is the data access object for the table plugin_idc_core_content_service_scope.
type ContentServiceScopeDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  ContentServiceScopeColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// ContentServiceScopeColumns defines and stores column names for the table plugin_idc_core_content_service_scope.
type ContentServiceScopeColumns struct {
	Id        string //
	TenantId  string //
	ContentNo string //
	ServiceNo string //
	CreatedAt string //
}

// contentServiceScopeColumns holds the columns for the table plugin_idc_core_content_service_scope.
var contentServiceScopeColumns = ContentServiceScopeColumns{
	Id:        "id",
	TenantId:  "tenant_id",
	ContentNo: "content_no",
	ServiceNo: "service_no",
	CreatedAt: "created_at",
}

// NewContentServiceScopeDao creates and returns a new DAO object for table data access.
func NewContentServiceScopeDao(handlers ...gdb.ModelHandler) *ContentServiceScopeDao {
	return &ContentServiceScopeDao{
		group:    "default",
		table:    "plugin_idc_core_content_service_scope",
		columns:  contentServiceScopeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentServiceScopeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentServiceScopeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentServiceScopeDao) Columns() ContentServiceScopeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentServiceScopeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentServiceScopeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentServiceScopeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
