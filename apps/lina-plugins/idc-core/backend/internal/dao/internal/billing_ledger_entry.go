// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingLedgerEntryDao is the data access object for the table plugin_idc_core_billing_ledger_entry.
type BillingLedgerEntryDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  BillingLedgerEntryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// BillingLedgerEntryColumns defines and stores column names for the table plugin_idc_core_billing_ledger_entry.
type BillingLedgerEntryColumns struct {
	Id            string //
	TenantId      string //
	EntryNo       string //
	TransactionNo string //
	CurrencyCode  string //
	LineNumber    string //
	AccountNo     string //
	DirectionCode string //
	EntryRoleCode string //
	AmountMinor   string //
	InvoiceNo     string //
	InvoiceItemNo string //
	ReasonCode    string //
	ContentDigest string //
	CreatedAt     string //
}

// billingLedgerEntryColumns holds the columns for the table plugin_idc_core_billing_ledger_entry.
var billingLedgerEntryColumns = BillingLedgerEntryColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	EntryNo:       "entry_no",
	TransactionNo: "transaction_no",
	CurrencyCode:  "currency_code",
	LineNumber:    "line_number",
	AccountNo:     "account_no",
	DirectionCode: "direction_code",
	EntryRoleCode: "entry_role_code",
	AmountMinor:   "amount_minor",
	InvoiceNo:     "invoice_no",
	InvoiceItemNo: "invoice_item_no",
	ReasonCode:    "reason_code",
	ContentDigest: "content_digest",
	CreatedAt:     "created_at",
}

// NewBillingLedgerEntryDao creates and returns a new DAO object for table data access.
func NewBillingLedgerEntryDao(handlers ...gdb.ModelHandler) *BillingLedgerEntryDao {
	return &BillingLedgerEntryDao{
		group:    "default",
		table:    "plugin_idc_core_billing_ledger_entry",
		columns:  billingLedgerEntryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingLedgerEntryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingLedgerEntryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingLedgerEntryDao) Columns() BillingLedgerEntryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingLedgerEntryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingLedgerEntryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingLedgerEntryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
