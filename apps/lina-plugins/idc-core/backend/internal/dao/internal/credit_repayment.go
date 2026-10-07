// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditRepaymentDao is the data access object for the table plugin_idc_core_credit_repayment.
type CreditRepaymentDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  CreditRepaymentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// CreditRepaymentColumns defines and stores column names for the table plugin_idc_core_credit_repayment.
type CreditRepaymentColumns struct {
	Id                         string //
	TenantId                   string //
	RepaymentNo                string //
	CreditAccountNo            string //
	CustomerNo                 string //
	CurrencyCode               string //
	RepaymentTypeCode          string //
	FundingSourceTypeCode      string //
	FundingSourceReference     string //
	LedgerTransactionNo        string //
	AmountMinor                string //
	AllocatedAmountMinor       string //
	UnallocatedAmountMinor     string //
	RefundedAmountMinor        string //
	StatusCode                 string //
	SourceSnapshotDigest       string //
	ConfirmationEvidenceDigest string //
	ConfirmedAt                string //
	FailedAt                   string //
	RefundedAt                 string //
	Version                    string //
	CreatedAt                  string //
	UpdatedAt                  string //
}

// creditRepaymentColumns holds the columns for the table plugin_idc_core_credit_repayment.
var creditRepaymentColumns = CreditRepaymentColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	RepaymentNo:                "repayment_no",
	CreditAccountNo:            "credit_account_no",
	CustomerNo:                 "customer_no",
	CurrencyCode:               "currency_code",
	RepaymentTypeCode:          "repayment_type_code",
	FundingSourceTypeCode:      "funding_source_type_code",
	FundingSourceReference:     "funding_source_reference",
	LedgerTransactionNo:        "ledger_transaction_no",
	AmountMinor:                "amount_minor",
	AllocatedAmountMinor:       "allocated_amount_minor",
	UnallocatedAmountMinor:     "unallocated_amount_minor",
	RefundedAmountMinor:        "refunded_amount_minor",
	StatusCode:                 "status_code",
	SourceSnapshotDigest:       "source_snapshot_digest",
	ConfirmationEvidenceDigest: "confirmation_evidence_digest",
	ConfirmedAt:                "confirmed_at",
	FailedAt:                   "failed_at",
	RefundedAt:                 "refunded_at",
	Version:                    "version",
	CreatedAt:                  "created_at",
	UpdatedAt:                  "updated_at",
}

// NewCreditRepaymentDao creates and returns a new DAO object for table data access.
func NewCreditRepaymentDao(handlers ...gdb.ModelHandler) *CreditRepaymentDao {
	return &CreditRepaymentDao{
		group:    "default",
		table:    "plugin_idc_core_credit_repayment",
		columns:  creditRepaymentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditRepaymentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditRepaymentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditRepaymentDao) Columns() CreditRepaymentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditRepaymentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditRepaymentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditRepaymentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
