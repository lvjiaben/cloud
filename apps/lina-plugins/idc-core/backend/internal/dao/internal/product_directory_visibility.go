// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductDirectoryVisibilityDao is the data access object for the table plugin_idc_core_product_directory_visibility.
type ProductDirectoryVisibilityDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  ProductDirectoryVisibilityColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// ProductDirectoryVisibilityColumns defines and stores column names for the table plugin_idc_core_product_directory_visibility.
type ProductDirectoryVisibilityColumns struct {
	TenantId    string //
	CustomerNo  string //
	DirectoryNo string //
	Hidden      string //
	UpdatedAt   string //
}

// productDirectoryVisibilityColumns holds the columns for the table plugin_idc_core_product_directory_visibility.
var productDirectoryVisibilityColumns = ProductDirectoryVisibilityColumns{
	TenantId:    "tenant_id",
	CustomerNo:  "customer_no",
	DirectoryNo: "directory_no",
	Hidden:      "hidden",
	UpdatedAt:   "updated_at",
}

// NewProductDirectoryVisibilityDao creates and returns a new DAO object for table data access.
func NewProductDirectoryVisibilityDao(handlers ...gdb.ModelHandler) *ProductDirectoryVisibilityDao {
	return &ProductDirectoryVisibilityDao{
		group:    "default",
		table:    "plugin_idc_core_product_directory_visibility",
		columns:  productDirectoryVisibilityColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductDirectoryVisibilityDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductDirectoryVisibilityDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductDirectoryVisibilityDao) Columns() ProductDirectoryVisibilityColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductDirectoryVisibilityDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductDirectoryVisibilityDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductDirectoryVisibilityDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
