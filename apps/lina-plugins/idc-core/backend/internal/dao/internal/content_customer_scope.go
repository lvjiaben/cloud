// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentCustomerScopeDao is the data access object for the table plugin_idc_core_content_customer_scope.
type ContentCustomerScopeDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ContentCustomerScopeColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ContentCustomerScopeColumns defines and stores column names for the table plugin_idc_core_content_customer_scope.
type ContentCustomerScopeColumns struct {
	Id         string //
	TenantId   string //
	ContentNo  string //
	CustomerNo string //
	CreatedAt  string //
}

// contentCustomerScopeColumns holds the columns for the table plugin_idc_core_content_customer_scope.
var contentCustomerScopeColumns = ContentCustomerScopeColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	ContentNo:  "content_no",
	CustomerNo: "customer_no",
	CreatedAt:  "created_at",
}

// NewContentCustomerScopeDao creates and returns a new DAO object for table data access.
func NewContentCustomerScopeDao(handlers ...gdb.ModelHandler) *ContentCustomerScopeDao {
	return &ContentCustomerScopeDao{
		group:    "default",
		table:    "plugin_idc_core_content_customer_scope",
		columns:  contentCustomerScopeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentCustomerScopeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentCustomerScopeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentCustomerScopeDao) Columns() ContentCustomerScopeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentCustomerScopeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentCustomerScopeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentCustomerScopeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
