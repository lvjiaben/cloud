// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldDao is the data access object for the table plugin_idc_core_customer_custom_field.
type CustomerCustomFieldDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  CustomerCustomFieldColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// CustomerCustomFieldColumns defines and stores column names for the table plugin_idc_core_customer_custom_field.
type CustomerCustomFieldColumns struct {
	Id                string //
	TenantId          string //
	FieldNo           string //
	FieldKey          string //
	LabelZhCn         string //
	LabelEnUs         string //
	HelpZhCn          string //
	HelpEnUs          string //
	DataTypeCode      string //
	CustomerTypeCode  string //
	VisibilityCode    string //
	SensitivityCode   string // Controls masking, export, audit, and AI exclusion policy
	Required          string //
	CustomerEditable  string //
	MinimumLength     string //
	MaximumLength     string //
	MinimumNumber     string //
	MaximumNumber     string //
	ValidationPattern string // Bounded service-validated pattern; never evaluated by an unbounded backtracking engine
	Priority          string //
	StatusCode        string //
	Version           string //
	CreatedBy         string //
	UpdatedBy         string //
	CreatedAt         string //
	UpdatedAt         string //
}

// customerCustomFieldColumns holds the columns for the table plugin_idc_core_customer_custom_field.
var customerCustomFieldColumns = CustomerCustomFieldColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	FieldNo:           "field_no",
	FieldKey:          "field_key",
	LabelZhCn:         "label_zh_cn",
	LabelEnUs:         "label_en_us",
	HelpZhCn:          "help_zh_cn",
	HelpEnUs:          "help_en_us",
	DataTypeCode:      "data_type_code",
	CustomerTypeCode:  "customer_type_code",
	VisibilityCode:    "visibility_code",
	SensitivityCode:   "sensitivity_code",
	Required:          "required",
	CustomerEditable:  "customer_editable",
	MinimumLength:     "minimum_length",
	MaximumLength:     "maximum_length",
	MinimumNumber:     "minimum_number",
	MaximumNumber:     "maximum_number",
	ValidationPattern: "validation_pattern",
	Priority:          "priority",
	StatusCode:        "status_code",
	Version:           "version",
	CreatedBy:         "created_by",
	UpdatedBy:         "updated_by",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewCustomerCustomFieldDao creates and returns a new DAO object for table data access.
func NewCustomerCustomFieldDao(handlers ...gdb.ModelHandler) *CustomerCustomFieldDao {
	return &CustomerCustomFieldDao{
		group:    "default",
		table:    "plugin_idc_core_customer_custom_field",
		columns:  customerCustomFieldColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerCustomFieldDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerCustomFieldDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerCustomFieldDao) Columns() CustomerCustomFieldColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerCustomFieldDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerCustomFieldDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerCustomFieldDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
