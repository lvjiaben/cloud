// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductDirectoryPageDao is the data access object for the table plugin_idc_core_product_directory_page.
type ProductDirectoryPageDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ProductDirectoryPageColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ProductDirectoryPageColumns defines and stores column names for the table plugin_idc_core_product_directory_page.
type ProductDirectoryPageColumns struct {
	TenantId    string //
	DirectoryNo string //
	PluginId    string //
	SortOrder   string //
}

// productDirectoryPageColumns holds the columns for the table plugin_idc_core_product_directory_page.
var productDirectoryPageColumns = ProductDirectoryPageColumns{
	TenantId:    "tenant_id",
	DirectoryNo: "directory_no",
	PluginId:    "plugin_id",
	SortOrder:   "sort_order",
}

// NewProductDirectoryPageDao creates and returns a new DAO object for table data access.
func NewProductDirectoryPageDao(handlers ...gdb.ModelHandler) *ProductDirectoryPageDao {
	return &ProductDirectoryPageDao{
		group:    "default",
		table:    "plugin_idc_core_product_directory_page",
		columns:  productDirectoryPageColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductDirectoryPageDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductDirectoryPageDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductDirectoryPageDao) Columns() ProductDirectoryPageColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductDirectoryPageDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductDirectoryPageDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductDirectoryPageDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
