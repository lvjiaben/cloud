// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerTagDao is the data access object for the table plugin_idc_core_customer_tag.
type CustomerTagDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  CustomerTagColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// CustomerTagColumns defines and stores column names for the table plugin_idc_core_customer_tag.
type CustomerTagColumns struct {
	Id         string //
	TenantId   string //
	TagNo      string //
	TagCode    string //
	NameZhCn   string //
	NameEnUs   string //
	ColorToken string //
	StatusCode string //
	Version    string //
	CreatedBy  string //
	UpdatedBy  string //
	CreatedAt  string //
	UpdatedAt  string //
}

// customerTagColumns holds the columns for the table plugin_idc_core_customer_tag.
var customerTagColumns = CustomerTagColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	TagNo:      "tag_no",
	TagCode:    "tag_code",
	NameZhCn:   "name_zh_cn",
	NameEnUs:   "name_en_us",
	ColorToken: "color_token",
	StatusCode: "status_code",
	Version:    "version",
	CreatedBy:  "created_by",
	UpdatedBy:  "updated_by",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewCustomerTagDao creates and returns a new DAO object for table data access.
func NewCustomerTagDao(handlers ...gdb.ModelHandler) *CustomerTagDao {
	return &CustomerTagDao{
		group:    "default",
		table:    "plugin_idc_core_customer_tag",
		columns:  customerTagColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerTagDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerTagDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerTagDao) Columns() CustomerTagColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerTagDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerTagDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerTagDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
