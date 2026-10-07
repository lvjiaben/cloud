// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceTransformSourceDao is the data access object for the table plugin_idc_core_billing_invoice_transform_source.
type BillingInvoiceTransformSourceDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  BillingInvoiceTransformSourceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// BillingInvoiceTransformSourceColumns defines and stores column names for the table plugin_idc_core_billing_invoice_transform_source.
type BillingInvoiceTransformSourceColumns struct {
	Id                  string //
	TenantId            string //
	TransformNo         string //
	SourceInvoiceNo     string //
	CustomerNo          string //
	CurrencyCode        string //
	Position            string //
	SourceContentDigest string //
	ContentDigest       string //
	CreatedAt           string //
}

// billingInvoiceTransformSourceColumns holds the columns for the table plugin_idc_core_billing_invoice_transform_source.
var billingInvoiceTransformSourceColumns = BillingInvoiceTransformSourceColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	TransformNo:         "transform_no",
	SourceInvoiceNo:     "source_invoice_no",
	CustomerNo:          "customer_no",
	CurrencyCode:        "currency_code",
	Position:            "position",
	SourceContentDigest: "source_content_digest",
	ContentDigest:       "content_digest",
	CreatedAt:           "created_at",
}

// NewBillingInvoiceTransformSourceDao creates and returns a new DAO object for table data access.
func NewBillingInvoiceTransformSourceDao(handlers ...gdb.ModelHandler) *BillingInvoiceTransformSourceDao {
	return &BillingInvoiceTransformSourceDao{
		group:    "default",
		table:    "plugin_idc_core_billing_invoice_transform_source",
		columns:  billingInvoiceTransformSourceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingInvoiceTransformSourceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingInvoiceTransformSourceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingInvoiceTransformSourceDao) Columns() BillingInvoiceTransformSourceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingInvoiceTransformSourceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingInvoiceTransformSourceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingInvoiceTransformSourceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
