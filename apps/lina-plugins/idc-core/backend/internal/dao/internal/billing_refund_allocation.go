// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingRefundAllocationDao is the data access object for the table plugin_idc_core_billing_refund_allocation.
type BillingRefundAllocationDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  BillingRefundAllocationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// BillingRefundAllocationColumns defines and stores column names for the table plugin_idc_core_billing_refund_allocation.
type BillingRefundAllocationColumns struct {
	Id                         string //
	TenantId                   string //
	RefundAllocationNo         string //
	InvoiceNo                  string //
	InvoiceItemNo              string //
	CustomerNo                 string //
	CurrencyCode               string //
	PaymentAllocationNo        string //
	RefundSourceTypeCode       string //
	RefundSourceReference      string //
	LedgerTransactionNo        string //
	LedgerTransactionTypeCode  string //
	DirectionCode              string //
	OriginalRefundAllocationNo string //
	AmountMinor                string //
	ContentDigest              string //
	CreatedAt                  string //
}

// billingRefundAllocationColumns holds the columns for the table plugin_idc_core_billing_refund_allocation.
var billingRefundAllocationColumns = BillingRefundAllocationColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	RefundAllocationNo:         "refund_allocation_no",
	InvoiceNo:                  "invoice_no",
	InvoiceItemNo:              "invoice_item_no",
	CustomerNo:                 "customer_no",
	CurrencyCode:               "currency_code",
	PaymentAllocationNo:        "payment_allocation_no",
	RefundSourceTypeCode:       "refund_source_type_code",
	RefundSourceReference:      "refund_source_reference",
	LedgerTransactionNo:        "ledger_transaction_no",
	LedgerTransactionTypeCode:  "ledger_transaction_type_code",
	DirectionCode:              "direction_code",
	OriginalRefundAllocationNo: "original_refund_allocation_no",
	AmountMinor:                "amount_minor",
	ContentDigest:              "content_digest",
	CreatedAt:                  "created_at",
}

// NewBillingRefundAllocationDao creates and returns a new DAO object for table data access.
func NewBillingRefundAllocationDao(handlers ...gdb.ModelHandler) *BillingRefundAllocationDao {
	return &BillingRefundAllocationDao{
		group:    "default",
		table:    "plugin_idc_core_billing_refund_allocation",
		columns:  billingRefundAllocationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingRefundAllocationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingRefundAllocationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingRefundAllocationDao) Columns() BillingRefundAllocationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingRefundAllocationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingRefundAllocationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingRefundAllocationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
