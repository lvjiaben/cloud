// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceAllocationDao is the data access object for the table plugin_idc_compliance_tax_invoice_allocation.
type TaxInvoiceAllocationDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  TaxInvoiceAllocationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// TaxInvoiceAllocationColumns defines and stores column names for the table plugin_idc_compliance_tax_invoice_allocation.
type TaxInvoiceAllocationColumns struct {
	Id                    string //
	TenantId              string //
	AllocationNo          string //
	TaxInvoiceNo          string //
	BillingInvoiceNo      string // Opaque reference to a core receivable; no cross-plugin foreign key is permitted
	OrderNo               string //
	AllocatedGrossMinor   string //
	AllocatedNetMinor     string //
	AllocatedTaxMinor     string //
	RefundedGrossMinor    string //
	RedLetteredGrossMinor string //
	CreatedAt             string //
	UpdatedAt             string //
}

// taxInvoiceAllocationColumns holds the columns for the table plugin_idc_compliance_tax_invoice_allocation.
var taxInvoiceAllocationColumns = TaxInvoiceAllocationColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	AllocationNo:          "allocation_no",
	TaxInvoiceNo:          "tax_invoice_no",
	BillingInvoiceNo:      "billing_invoice_no",
	OrderNo:               "order_no",
	AllocatedGrossMinor:   "allocated_gross_minor",
	AllocatedNetMinor:     "allocated_net_minor",
	AllocatedTaxMinor:     "allocated_tax_minor",
	RefundedGrossMinor:    "refunded_gross_minor",
	RedLetteredGrossMinor: "red_lettered_gross_minor",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
}

// NewTaxInvoiceAllocationDao creates and returns a new DAO object for table data access.
func NewTaxInvoiceAllocationDao(handlers ...gdb.ModelHandler) *TaxInvoiceAllocationDao {
	return &TaxInvoiceAllocationDao{
		group:    "default",
		table:    "plugin_idc_compliance_tax_invoice_allocation",
		columns:  taxInvoiceAllocationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaxInvoiceAllocationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaxInvoiceAllocationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaxInvoiceAllocationDao) Columns() TaxInvoiceAllocationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaxInvoiceAllocationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaxInvoiceAllocationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaxInvoiceAllocationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
