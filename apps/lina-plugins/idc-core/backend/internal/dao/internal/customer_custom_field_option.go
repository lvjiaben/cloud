// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldOptionDao is the data access object for the table plugin_idc_core_customer_custom_field_option.
type CustomerCustomFieldOptionDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CustomerCustomFieldOptionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CustomerCustomFieldOptionColumns defines and stores column names for the table plugin_idc_core_customer_custom_field_option.
type CustomerCustomFieldOptionColumns struct {
	Id          string //
	TenantId    string //
	OptionNo    string //
	FieldNo     string //
	OptionValue string //
	LabelZhCn   string //
	LabelEnUs   string //
	Priority    string //
	StatusCode  string //
	CreatedAt   string //
	UpdatedAt   string //
}

// customerCustomFieldOptionColumns holds the columns for the table plugin_idc_core_customer_custom_field_option.
var customerCustomFieldOptionColumns = CustomerCustomFieldOptionColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	OptionNo:    "option_no",
	FieldNo:     "field_no",
	OptionValue: "option_value",
	LabelZhCn:   "label_zh_cn",
	LabelEnUs:   "label_en_us",
	Priority:    "priority",
	StatusCode:  "status_code",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewCustomerCustomFieldOptionDao creates and returns a new DAO object for table data access.
func NewCustomerCustomFieldOptionDao(handlers ...gdb.ModelHandler) *CustomerCustomFieldOptionDao {
	return &CustomerCustomFieldOptionDao{
		group:    "default",
		table:    "plugin_idc_core_customer_custom_field_option",
		columns:  customerCustomFieldOptionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerCustomFieldOptionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerCustomFieldOptionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerCustomFieldOptionDao) Columns() CustomerCustomFieldOptionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerCustomFieldOptionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerCustomFieldOptionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerCustomFieldOptionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
