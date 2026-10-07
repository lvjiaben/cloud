// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceDao is the data access object for the table plugin_idc_core_billing_invoice.
type BillingInvoiceDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  BillingInvoiceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// BillingInvoiceColumns defines and stores column names for the table plugin_idc_core_billing_invoice.
type BillingInvoiceColumns struct {
	Id                    string //
	TenantId              string //
	InvoiceNo             string //
	CustomerNo            string //
	OrderNo               string //
	SourceCode            string //
	CurrencyCode          string //
	StatusCode            string //
	ItemCount             string //
	SubtotalMinor         string //
	DiscountMinor         string //
	NetAmountMinor        string //
	TaxAmountMinor        string //
	TotalAmountMinor      string //
	PaidAmountMinor       string //
	RefundableAmountMinor string //
	RefundedAmountMinor   string //
	SourceSnapshotDigest  string //
	ContentDigest         string //
	PaymentDueAt          string //
	IssuedAt              string //
	SettledAt             string //
	CancelledAt           string //
	Version               string //
	CreatedAt             string //
	UpdatedAt             string //
	WaivedAmountMinor     string // Effective reviewed waiver total; never counted as customer payment
	CreditAmountMinor     string //
}

// billingInvoiceColumns holds the columns for the table plugin_idc_core_billing_invoice.
var billingInvoiceColumns = BillingInvoiceColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	InvoiceNo:             "invoice_no",
	CustomerNo:            "customer_no",
	OrderNo:               "order_no",
	SourceCode:            "source_code",
	CurrencyCode:          "currency_code",
	StatusCode:            "status_code",
	ItemCount:             "item_count",
	SubtotalMinor:         "subtotal_minor",
	DiscountMinor:         "discount_minor",
	NetAmountMinor:        "net_amount_minor",
	TaxAmountMinor:        "tax_amount_minor",
	TotalAmountMinor:      "total_amount_minor",
	PaidAmountMinor:       "paid_amount_minor",
	RefundableAmountMinor: "refundable_amount_minor",
	RefundedAmountMinor:   "refunded_amount_minor",
	SourceSnapshotDigest:  "source_snapshot_digest",
	ContentDigest:         "content_digest",
	PaymentDueAt:          "payment_due_at",
	IssuedAt:              "issued_at",
	SettledAt:             "settled_at",
	CancelledAt:           "cancelled_at",
	Version:               "version",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	WaivedAmountMinor:     "waived_amount_minor",
	CreditAmountMinor:     "credit_amount_minor",
}

// NewBillingInvoiceDao creates and returns a new DAO object for table data access.
func NewBillingInvoiceDao(handlers ...gdb.ModelHandler) *BillingInvoiceDao {
	return &BillingInvoiceDao{
		group:    "default",
		table:    "plugin_idc_core_billing_invoice",
		columns:  billingInvoiceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingInvoiceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingInvoiceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingInvoiceDao) Columns() BillingInvoiceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingInvoiceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingInvoiceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingInvoiceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
