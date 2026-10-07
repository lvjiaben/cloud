// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceItemDao is the data access object for the table plugin_idc_core_billing_invoice_item.
type BillingInvoiceItemDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  BillingInvoiceItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// BillingInvoiceItemColumns defines and stores column names for the table plugin_idc_core_billing_invoice_item.
type BillingInvoiceItemColumns struct {
	Id                 string //
	TenantId           string //
	InvoiceItemNo      string //
	InvoiceNo          string //
	CustomerNo         string //
	CurrencyCode       string //
	LineNumber         string //
	ItemTypeCode       string //
	OrderNo            string //
	OrderItemNo        string //
	SourceReference    string //
	DescriptionZhCn    string //
	DescriptionEnUs    string //
	SubtotalMinor      string //
	DiscountMinor      string //
	NetAmountMinor     string //
	TaxAmountMinor     string //
	TotalAmountMinor   string //
	RefundableCapMinor string //
	ContentDigest      string //
	CreatedAt          string //
}

// billingInvoiceItemColumns holds the columns for the table plugin_idc_core_billing_invoice_item.
var billingInvoiceItemColumns = BillingInvoiceItemColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	InvoiceItemNo:      "invoice_item_no",
	InvoiceNo:          "invoice_no",
	CustomerNo:         "customer_no",
	CurrencyCode:       "currency_code",
	LineNumber:         "line_number",
	ItemTypeCode:       "item_type_code",
	OrderNo:            "order_no",
	OrderItemNo:        "order_item_no",
	SourceReference:    "source_reference",
	DescriptionZhCn:    "description_zh_cn",
	DescriptionEnUs:    "description_en_us",
	SubtotalMinor:      "subtotal_minor",
	DiscountMinor:      "discount_minor",
	NetAmountMinor:     "net_amount_minor",
	TaxAmountMinor:     "tax_amount_minor",
	TotalAmountMinor:   "total_amount_minor",
	RefundableCapMinor: "refundable_cap_minor",
	ContentDigest:      "content_digest",
	CreatedAt:          "created_at",
}

// NewBillingInvoiceItemDao creates and returns a new DAO object for table data access.
func NewBillingInvoiceItemDao(handlers ...gdb.ModelHandler) *BillingInvoiceItemDao {
	return &BillingInvoiceItemDao{
		group:    "default",
		table:    "plugin_idc_core_billing_invoice_item",
		columns:  billingInvoiceItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingInvoiceItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingInvoiceItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingInvoiceItemDao) Columns() BillingInvoiceItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingInvoiceItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingInvoiceItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingInvoiceItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
