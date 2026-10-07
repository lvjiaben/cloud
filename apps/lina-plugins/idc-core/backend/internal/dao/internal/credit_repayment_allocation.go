// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditRepaymentAllocationDao is the data access object for the table plugin_idc_core_credit_repayment_allocation.
type CreditRepaymentAllocationDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CreditRepaymentAllocationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CreditRepaymentAllocationColumns defines and stores column names for the table plugin_idc_core_credit_repayment_allocation.
type CreditRepaymentAllocationColumns struct {
	Id                   string //
	TenantId             string //
	AllocationNo         string //
	RepaymentNo          string //
	CreditAccountNo      string //
	CustomerNo           string //
	CurrencyCode         string //
	StatementNo          string //
	ExposureNo           string //
	AllocationTypeCode   string //
	DirectionCode        string //
	AmountMinor          string //
	OriginalAllocationNo string //
	SourceReference      string //
	ContentDigest        string //
	CreatedAt            string //
}

// creditRepaymentAllocationColumns holds the columns for the table plugin_idc_core_credit_repayment_allocation.
var creditRepaymentAllocationColumns = CreditRepaymentAllocationColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	AllocationNo:         "allocation_no",
	RepaymentNo:          "repayment_no",
	CreditAccountNo:      "credit_account_no",
	CustomerNo:           "customer_no",
	CurrencyCode:         "currency_code",
	StatementNo:          "statement_no",
	ExposureNo:           "exposure_no",
	AllocationTypeCode:   "allocation_type_code",
	DirectionCode:        "direction_code",
	AmountMinor:          "amount_minor",
	OriginalAllocationNo: "original_allocation_no",
	SourceReference:      "source_reference",
	ContentDigest:        "content_digest",
	CreatedAt:            "created_at",
}

// NewCreditRepaymentAllocationDao creates and returns a new DAO object for table data access.
func NewCreditRepaymentAllocationDao(handlers ...gdb.ModelHandler) *CreditRepaymentAllocationDao {
	return &CreditRepaymentAllocationDao{
		group:    "default",
		table:    "plugin_idc_core_credit_repayment_allocation",
		columns:  creditRepaymentAllocationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditRepaymentAllocationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditRepaymentAllocationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditRepaymentAllocationDao) Columns() CreditRepaymentAllocationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditRepaymentAllocationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditRepaymentAllocationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditRepaymentAllocationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
