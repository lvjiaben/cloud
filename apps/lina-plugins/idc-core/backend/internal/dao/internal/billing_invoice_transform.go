// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceTransformDao is the data access object for the table plugin_idc_core_billing_invoice_transform.
type BillingInvoiceTransformDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  BillingInvoiceTransformColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// BillingInvoiceTransformColumns defines and stores column names for the table plugin_idc_core_billing_invoice_transform.
type BillingInvoiceTransformColumns struct {
	Id                string //
	TenantId          string //
	TransformNo       string //
	TransformTypeCode string //
	CustomerNo        string //
	CurrencyCode      string //
	SourceCount       string //
	DestinationCount  string //
	ItemCount         string //
	IdempotencyDigest string //
	RequestDigest     string //
	ContentDigest     string //
	RequestedBy       string //
	CompletedAt       string //
	CreatedAt         string //
}

// billingInvoiceTransformColumns holds the columns for the table plugin_idc_core_billing_invoice_transform.
var billingInvoiceTransformColumns = BillingInvoiceTransformColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	TransformNo:       "transform_no",
	TransformTypeCode: "transform_type_code",
	CustomerNo:        "customer_no",
	CurrencyCode:      "currency_code",
	SourceCount:       "source_count",
	DestinationCount:  "destination_count",
	ItemCount:         "item_count",
	IdempotencyDigest: "idempotency_digest",
	RequestDigest:     "request_digest",
	ContentDigest:     "content_digest",
	RequestedBy:       "requested_by",
	CompletedAt:       "completed_at",
	CreatedAt:         "created_at",
}

// NewBillingInvoiceTransformDao creates and returns a new DAO object for table data access.
func NewBillingInvoiceTransformDao(handlers ...gdb.ModelHandler) *BillingInvoiceTransformDao {
	return &BillingInvoiceTransformDao{
		group:    "default",
		table:    "plugin_idc_core_billing_invoice_transform",
		columns:  billingInvoiceTransformColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingInvoiceTransformDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingInvoiceTransformDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingInvoiceTransformDao) Columns() BillingInvoiceTransformColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingInvoiceTransformDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingInvoiceTransformDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingInvoiceTransformDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
