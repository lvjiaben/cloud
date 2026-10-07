// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingLedgerTransactionDao is the data access object for the table plugin_idc_core_billing_ledger_transaction.
type BillingLedgerTransactionDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  BillingLedgerTransactionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// BillingLedgerTransactionColumns defines and stores column names for the table plugin_idc_core_billing_ledger_transaction.
type BillingLedgerTransactionColumns struct {
	Id                    string //
	TenantId              string //
	TransactionNo         string //
	TransactionTypeCode   string //
	CurrencyCode          string //
	SourceTypeCode        string //
	SourceReference       string //
	SourceSnapshotDigest  string //
	OriginalTransactionNo string //
	EntryCount            string //
	DebitTotalMinor       string //
	CreditTotalMinor      string //
	ContentDigest         string //
	PostedBy              string //
	PostedAt              string //
	CreatedAt             string //
}

// billingLedgerTransactionColumns holds the columns for the table plugin_idc_core_billing_ledger_transaction.
var billingLedgerTransactionColumns = BillingLedgerTransactionColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	TransactionNo:         "transaction_no",
	TransactionTypeCode:   "transaction_type_code",
	CurrencyCode:          "currency_code",
	SourceTypeCode:        "source_type_code",
	SourceReference:       "source_reference",
	SourceSnapshotDigest:  "source_snapshot_digest",
	OriginalTransactionNo: "original_transaction_no",
	EntryCount:            "entry_count",
	DebitTotalMinor:       "debit_total_minor",
	CreditTotalMinor:      "credit_total_minor",
	ContentDigest:         "content_digest",
	PostedBy:              "posted_by",
	PostedAt:              "posted_at",
	CreatedAt:             "created_at",
}

// NewBillingLedgerTransactionDao creates and returns a new DAO object for table data access.
func NewBillingLedgerTransactionDao(handlers ...gdb.ModelHandler) *BillingLedgerTransactionDao {
	return &BillingLedgerTransactionDao{
		group:    "default",
		table:    "plugin_idc_core_billing_ledger_transaction",
		columns:  billingLedgerTransactionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingLedgerTransactionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingLedgerTransactionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingLedgerTransactionDao) Columns() BillingLedgerTransactionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingLedgerTransactionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingLedgerTransactionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingLedgerTransactionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
