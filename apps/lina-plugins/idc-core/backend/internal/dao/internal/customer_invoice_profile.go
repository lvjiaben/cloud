// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerInvoiceProfileDao is the data access object for the table plugin_idc_core_customer_invoice_profile.
type CustomerInvoiceProfileDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  CustomerInvoiceProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// CustomerInvoiceProfileColumns defines and stores column names for the table plugin_idc_core_customer_invoice_profile.
type CustomerInvoiceProfileColumns struct {
	Id                   string //
	TenantId             string //
	ProfileNo            string //
	CustomerNo           string //
	TitleTypeCode        string //
	InvoiceTitle         string //
	TaxRegistrationNo    string //
	RegisteredAddress    string //
	RegisteredPhone      string //
	BankName             string //
	BankAccountMasked    string //
	BankAccountSecretRef string // Opaque core secret-envelope reference; never returned by ordinary APIs or logs
	IsDefault            string //
	StatusCode           string //
	Version              string //
	CreatedBy            string //
	UpdatedBy            string //
	CreatedAt            string //
	UpdatedAt            string //
}

// customerInvoiceProfileColumns holds the columns for the table plugin_idc_core_customer_invoice_profile.
var customerInvoiceProfileColumns = CustomerInvoiceProfileColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	ProfileNo:            "profile_no",
	CustomerNo:           "customer_no",
	TitleTypeCode:        "title_type_code",
	InvoiceTitle:         "invoice_title",
	TaxRegistrationNo:    "tax_registration_no",
	RegisteredAddress:    "registered_address",
	RegisteredPhone:      "registered_phone",
	BankName:             "bank_name",
	BankAccountMasked:    "bank_account_masked",
	BankAccountSecretRef: "bank_account_secret_ref",
	IsDefault:            "is_default",
	StatusCode:           "status_code",
	Version:              "version",
	CreatedBy:            "created_by",
	UpdatedBy:            "updated_by",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewCustomerInvoiceProfileDao creates and returns a new DAO object for table data access.
func NewCustomerInvoiceProfileDao(handlers ...gdb.ModelHandler) *CustomerInvoiceProfileDao {
	return &CustomerInvoiceProfileDao{
		group:    "default",
		table:    "plugin_idc_core_customer_invoice_profile",
		columns:  customerInvoiceProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerInvoiceProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerInvoiceProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerInvoiceProfileDao) Columns() CustomerInvoiceProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerInvoiceProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerInvoiceProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerInvoiceProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
