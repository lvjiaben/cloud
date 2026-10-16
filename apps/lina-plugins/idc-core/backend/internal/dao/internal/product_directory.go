// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductDirectoryDao is the data access object for the table plugin_idc_core_product_directory.
type ProductDirectoryDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  ProductDirectoryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// ProductDirectoryColumns defines and stores column names for the table plugin_idc_core_product_directory.
type ProductDirectoryColumns struct {
	Id             string //
	TenantId       string //
	DirectoryNo    string //
	NameZhCn       string //
	NameEnUs       string //
	SortOrder      string //
	StatusCode     string //
	Version        string //
	CreatedBy      string //
	UpdatedBy      string //
	CreatedAt      string //
	UpdatedAt      string //
	NavigationType string //
	TargetUrl      string //
	Icon           string //
}

// productDirectoryColumns holds the columns for the table plugin_idc_core_product_directory.
var productDirectoryColumns = ProductDirectoryColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	DirectoryNo:    "directory_no",
	NameZhCn:       "name_zh_cn",
	NameEnUs:       "name_en_us",
	SortOrder:      "sort_order",
	StatusCode:     "status_code",
	Version:        "version",
	CreatedBy:      "created_by",
	UpdatedBy:      "updated_by",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
	NavigationType: "navigation_type",
	TargetUrl:      "target_url",
	Icon:           "icon",
}

// NewProductDirectoryDao creates and returns a new DAO object for table data access.
func NewProductDirectoryDao(handlers ...gdb.ModelHandler) *ProductDirectoryDao {
	return &ProductDirectoryDao{
		group:    "default",
		table:    "plugin_idc_core_product_directory",
		columns:  productDirectoryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductDirectoryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductDirectoryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductDirectoryDao) Columns() ProductDirectoryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductDirectoryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductDirectoryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductDirectoryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
