// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldValueOptionDao is the data access object for the table plugin_idc_core_customer_custom_field_value_option.
type CustomerCustomFieldValueOptionDao struct {
	table    string                                // table is the underlying table name of the DAO.
	group    string                                // group is the database configuration group name of the current DAO.
	columns  CustomerCustomFieldValueOptionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                    // handlers for customized model modification.
}

// CustomerCustomFieldValueOptionColumns defines and stores column names for the table plugin_idc_core_customer_custom_field_value_option.
type CustomerCustomFieldValueOptionColumns struct {
	Id         string //
	TenantId   string //
	CustomerNo string //
	FieldNo    string //
	ValueNo    string //
	OptionNo   string //
	CreatedAt  string //
}

// customerCustomFieldValueOptionColumns holds the columns for the table plugin_idc_core_customer_custom_field_value_option.
var customerCustomFieldValueOptionColumns = CustomerCustomFieldValueOptionColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	CustomerNo: "customer_no",
	FieldNo:    "field_no",
	ValueNo:    "value_no",
	OptionNo:   "option_no",
	CreatedAt:  "created_at",
}

// NewCustomerCustomFieldValueOptionDao creates and returns a new DAO object for table data access.
func NewCustomerCustomFieldValueOptionDao(handlers ...gdb.ModelHandler) *CustomerCustomFieldValueOptionDao {
	return &CustomerCustomFieldValueOptionDao{
		group:    "default",
		table:    "plugin_idc_core_customer_custom_field_value_option",
		columns:  customerCustomFieldValueOptionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerCustomFieldValueOptionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerCustomFieldValueOptionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerCustomFieldValueOptionDao) Columns() CustomerCustomFieldValueOptionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerCustomFieldValueOptionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerCustomFieldValueOptionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerCustomFieldValueOptionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
