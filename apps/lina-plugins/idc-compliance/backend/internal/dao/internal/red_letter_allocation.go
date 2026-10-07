// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterAllocationDao is the data access object for the table plugin_idc_compliance_red_letter_allocation.
type RedLetterAllocationDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  RedLetterAllocationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// RedLetterAllocationColumns defines and stores column names for the table plugin_idc_compliance_red_letter_allocation.
type RedLetterAllocationColumns struct {
	Id                     string //
	TenantId               string //
	AllocationNo           string //
	TaskNo                 string //
	TaxInvoiceNo           string //
	TaxInvoiceAllocationNo string //
	RefundAllocationNo     string //
	GrossAmountMinor       string //
	NetAmountMinor         string //
	TaxAmountMinor         string //
	CreatedAt              string //
	TaxInvoiceItemNo       string //
}

// redLetterAllocationColumns holds the columns for the table plugin_idc_compliance_red_letter_allocation.
var redLetterAllocationColumns = RedLetterAllocationColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	AllocationNo:           "allocation_no",
	TaskNo:                 "task_no",
	TaxInvoiceNo:           "tax_invoice_no",
	TaxInvoiceAllocationNo: "tax_invoice_allocation_no",
	RefundAllocationNo:     "refund_allocation_no",
	GrossAmountMinor:       "gross_amount_minor",
	NetAmountMinor:         "net_amount_minor",
	TaxAmountMinor:         "tax_amount_minor",
	CreatedAt:              "created_at",
	TaxInvoiceItemNo:       "tax_invoice_item_no",
}

// NewRedLetterAllocationDao creates and returns a new DAO object for table data access.
func NewRedLetterAllocationDao(handlers ...gdb.ModelHandler) *RedLetterAllocationDao {
	return &RedLetterAllocationDao{
		group:    "default",
		table:    "plugin_idc_compliance_red_letter_allocation",
		columns:  redLetterAllocationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RedLetterAllocationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RedLetterAllocationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RedLetterAllocationDao) Columns() RedLetterAllocationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RedLetterAllocationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RedLetterAllocationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RedLetterAllocationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
