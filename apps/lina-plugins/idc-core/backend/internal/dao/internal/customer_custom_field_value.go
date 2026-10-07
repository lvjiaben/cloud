// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldValueDao is the data access object for the table plugin_idc_core_customer_custom_field_value.
type CustomerCustomFieldValueDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  CustomerCustomFieldValueColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// CustomerCustomFieldValueColumns defines and stores column names for the table plugin_idc_core_customer_custom_field_value.
type CustomerCustomFieldValueColumns struct {
	Id               string //
	TenantId         string //
	ValueNo          string //
	CustomerNo       string //
	FieldNo          string //
	TextValue        string //
	TextLookupDigest string // Keyed normalized digest for exact filtering without indexing long plaintext
	SecretValueRef   string // Opaque core secret-envelope reference used instead of plaintext for sensitive custom values
	IntegerValue     string //
	DecimalValue     string //
	BooleanValue     string //
	DateValue        string //
	DatetimeValue    string //
	StatusCode       string //
	Version          string //
	CreatedBy        string //
	UpdatedBy        string //
	CreatedAt        string //
	UpdatedAt        string //
}

// customerCustomFieldValueColumns holds the columns for the table plugin_idc_core_customer_custom_field_value.
var customerCustomFieldValueColumns = CustomerCustomFieldValueColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	ValueNo:          "value_no",
	CustomerNo:       "customer_no",
	FieldNo:          "field_no",
	TextValue:        "text_value",
	TextLookupDigest: "text_lookup_digest",
	SecretValueRef:   "secret_value_ref",
	IntegerValue:     "integer_value",
	DecimalValue:     "decimal_value",
	BooleanValue:     "boolean_value",
	DateValue:        "date_value",
	DatetimeValue:    "datetime_value",
	StatusCode:       "status_code",
	Version:          "version",
	CreatedBy:        "created_by",
	UpdatedBy:        "updated_by",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewCustomerCustomFieldValueDao creates and returns a new DAO object for table data access.
func NewCustomerCustomFieldValueDao(handlers ...gdb.ModelHandler) *CustomerCustomFieldValueDao {
	return &CustomerCustomFieldValueDao{
		group:    "default",
		table:    "plugin_idc_core_customer_custom_field_value",
		columns:  customerCustomFieldValueColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerCustomFieldValueDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerCustomFieldValueDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerCustomFieldValueDao) Columns() CustomerCustomFieldValueColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerCustomFieldValueDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerCustomFieldValueDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerCustomFieldValueDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
