// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingBalanceSnapshotDao is the data access object for the table plugin_idc_core_billing_balance_snapshot.
type BillingBalanceSnapshotDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  BillingBalanceSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// BillingBalanceSnapshotColumns defines and stores column names for the table plugin_idc_core_billing_balance_snapshot.
type BillingBalanceSnapshotColumns struct {
	Id                  string //
	TenantId            string //
	SnapshotNo          string //
	AccountNo           string //
	CurrencyCode        string //
	NormalBalanceCode   string //
	TransactionNo       string //
	SequenceNo          string //
	PreviousSnapshotNo  string //
	PreviousSequenceNo  string //
	OpeningBalanceMinor string //
	DebitAmountMinor    string //
	CreditAmountMinor   string //
	ClosingBalanceMinor string //
	ContentDigest       string //
	CreatedAt           string //
}

// billingBalanceSnapshotColumns holds the columns for the table plugin_idc_core_billing_balance_snapshot.
var billingBalanceSnapshotColumns = BillingBalanceSnapshotColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	SnapshotNo:          "snapshot_no",
	AccountNo:           "account_no",
	CurrencyCode:        "currency_code",
	NormalBalanceCode:   "normal_balance_code",
	TransactionNo:       "transaction_no",
	SequenceNo:          "sequence_no",
	PreviousSnapshotNo:  "previous_snapshot_no",
	PreviousSequenceNo:  "previous_sequence_no",
	OpeningBalanceMinor: "opening_balance_minor",
	DebitAmountMinor:    "debit_amount_minor",
	CreditAmountMinor:   "credit_amount_minor",
	ClosingBalanceMinor: "closing_balance_minor",
	ContentDigest:       "content_digest",
	CreatedAt:           "created_at",
}

// NewBillingBalanceSnapshotDao creates and returns a new DAO object for table data access.
func NewBillingBalanceSnapshotDao(handlers ...gdb.ModelHandler) *BillingBalanceSnapshotDao {
	return &BillingBalanceSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_billing_balance_snapshot",
		columns:  billingBalanceSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingBalanceSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingBalanceSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingBalanceSnapshotDao) Columns() BillingBalanceSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingBalanceSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingBalanceSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingBalanceSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
