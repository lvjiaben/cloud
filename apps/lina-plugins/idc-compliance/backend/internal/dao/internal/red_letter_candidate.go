// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterCandidateDao is the data access object for the table plugin_idc_compliance_red_letter_candidate.
type RedLetterCandidateDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  RedLetterCandidateColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// RedLetterCandidateColumns defines and stores column names for the table plugin_idc_compliance_red_letter_candidate.
type RedLetterCandidateColumns struct {
	Id                     string //
	TenantId               string //
	CandidateNo            string //
	TaskNo                 string //
	RefundAllocationNo     string //
	TaxInvoiceNo           string //
	TaxInvoiceAllocationNo string //
	TaxInvoiceItemNo       string //
	RequestedGrossMinor    string //
	AvailableGrossMinor    string //
	AvailableNetMinor      string //
	AvailableTaxMinor      string //
	CreatedAt              string //
}

// redLetterCandidateColumns holds the columns for the table plugin_idc_compliance_red_letter_candidate.
var redLetterCandidateColumns = RedLetterCandidateColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	CandidateNo:            "candidate_no",
	TaskNo:                 "task_no",
	RefundAllocationNo:     "refund_allocation_no",
	TaxInvoiceNo:           "tax_invoice_no",
	TaxInvoiceAllocationNo: "tax_invoice_allocation_no",
	TaxInvoiceItemNo:       "tax_invoice_item_no",
	RequestedGrossMinor:    "requested_gross_minor",
	AvailableGrossMinor:    "available_gross_minor",
	AvailableNetMinor:      "available_net_minor",
	AvailableTaxMinor:      "available_tax_minor",
	CreatedAt:              "created_at",
}

// NewRedLetterCandidateDao creates and returns a new DAO object for table data access.
func NewRedLetterCandidateDao(handlers ...gdb.ModelHandler) *RedLetterCandidateDao {
	return &RedLetterCandidateDao{
		group:    "default",
		table:    "plugin_idc_compliance_red_letter_candidate",
		columns:  redLetterCandidateColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RedLetterCandidateDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RedLetterCandidateDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RedLetterCandidateDao) Columns() RedLetterCandidateColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RedLetterCandidateDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RedLetterCandidateDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RedLetterCandidateDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
