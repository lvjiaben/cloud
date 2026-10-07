// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesGroupDao is the data access object for the table plugin_idc_core_sales_group.
type SalesGroupDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SalesGroupColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SalesGroupColumns defines and stores column names for the table plugin_idc_core_sales_group.
type SalesGroupColumns struct {
	Id         string //
	TenantId   string //
	GroupNo    string //
	GroupCode  string //
	NameZhCn   string //
	NameEnUs   string //
	StatusCode string //
	Version    string //
	CreatedAt  string //
	UpdatedAt  string //
}

// salesGroupColumns holds the columns for the table plugin_idc_core_sales_group.
var salesGroupColumns = SalesGroupColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	GroupNo:    "group_no",
	GroupCode:  "group_code",
	NameZhCn:   "name_zh_cn",
	NameEnUs:   "name_en_us",
	StatusCode: "status_code",
	Version:    "version",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewSalesGroupDao creates and returns a new DAO object for table data access.
func NewSalesGroupDao(handlers ...gdb.ModelHandler) *SalesGroupDao {
	return &SalesGroupDao{
		group:    "default",
		table:    "plugin_idc_core_sales_group",
		columns:  salesGroupColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesGroupDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesGroupDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesGroupDao) Columns() SalesGroupColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesGroupDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesGroupDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesGroupDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
