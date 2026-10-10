// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductDirectoryMembershipDao is the data access object for the table plugin_idc_core_product_directory_membership.
type ProductDirectoryMembershipDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  ProductDirectoryMembershipColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// ProductDirectoryMembershipColumns defines and stores column names for the table plugin_idc_core_product_directory_membership.
type ProductDirectoryMembershipColumns struct {
	TenantId    string //
	ProductNo   string //
	DirectoryNo string //
}

// productDirectoryMembershipColumns holds the columns for the table plugin_idc_core_product_directory_membership.
var productDirectoryMembershipColumns = ProductDirectoryMembershipColumns{
	TenantId:    "tenant_id",
	ProductNo:   "product_no",
	DirectoryNo: "directory_no",
}

// NewProductDirectoryMembershipDao creates and returns a new DAO object for table data access.
func NewProductDirectoryMembershipDao(handlers ...gdb.ModelHandler) *ProductDirectoryMembershipDao {
	return &ProductDirectoryMembershipDao{
		group:    "default",
		table:    "plugin_idc_core_product_directory_membership",
		columns:  productDirectoryMembershipColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductDirectoryMembershipDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductDirectoryMembershipDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductDirectoryMembershipDao) Columns() ProductDirectoryMembershipColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductDirectoryMembershipDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductDirectoryMembershipDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductDirectoryMembershipDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
