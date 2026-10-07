// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingPaymentAllocationDao is the data access object for the table plugin_idc_core_billing_payment_allocation.
type BillingPaymentAllocationDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  BillingPaymentAllocationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// BillingPaymentAllocationColumns defines and stores column names for the table plugin_idc_core_billing_payment_allocation.
type BillingPaymentAllocationColumns struct {
	Id                        string //
	TenantId                  string //
	AllocationNo              string //
	InvoiceNo                 string //
	InvoiceItemNo             string //
	CustomerNo                string //
	CurrencyCode              string //
	PaymentSourceTypeCode     string //
	PaymentSourceReference    string //
	LedgerTransactionNo       string //
	LedgerTransactionTypeCode string //
	DirectionCode             string //
	OriginalAllocationNo      string //
	AmountMinor               string //
	ContentDigest             string //
	CreatedAt                 string //
}

// billingPaymentAllocationColumns holds the columns for the table plugin_idc_core_billing_payment_allocation.
var billingPaymentAllocationColumns = BillingPaymentAllocationColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	AllocationNo:              "allocation_no",
	InvoiceNo:                 "invoice_no",
	InvoiceItemNo:             "invoice_item_no",
	CustomerNo:                "customer_no",
	CurrencyCode:              "currency_code",
	PaymentSourceTypeCode:     "payment_source_type_code",
	PaymentSourceReference:    "payment_source_reference",
	LedgerTransactionNo:       "ledger_transaction_no",
	LedgerTransactionTypeCode: "ledger_transaction_type_code",
	DirectionCode:             "direction_code",
	OriginalAllocationNo:      "original_allocation_no",
	AmountMinor:               "amount_minor",
	ContentDigest:             "content_digest",
	CreatedAt:                 "created_at",
}

// NewBillingPaymentAllocationDao creates and returns a new DAO object for table data access.
func NewBillingPaymentAllocationDao(handlers ...gdb.ModelHandler) *BillingPaymentAllocationDao {
	return &BillingPaymentAllocationDao{
		group:    "default",
		table:    "plugin_idc_core_billing_payment_allocation",
		columns:  billingPaymentAllocationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingPaymentAllocationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingPaymentAllocationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingPaymentAllocationDao) Columns() BillingPaymentAllocationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingPaymentAllocationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingPaymentAllocationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingPaymentAllocationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
