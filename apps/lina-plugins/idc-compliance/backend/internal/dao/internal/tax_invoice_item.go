// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceItemDao is the data access object for the table plugin_idc_compliance_tax_invoice_item.
type TaxInvoiceItemDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  TaxInvoiceItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// TaxInvoiceItemColumns defines and stores column names for the table plugin_idc_compliance_tax_invoice_item.
type TaxInvoiceItemColumns struct {
	Id                    string //
	TenantId              string //
	ItemNo                string //
	TaxInvoiceNo          string //
	LineNo                string //
	ItemSnapshotRef       string //
	ItemSnapshotSha256    string //
	TaxClassificationCode string //
	TaxRatePpm            string //
	GrossAmountMinor      string //
	NetAmountMinor        string //
	TaxAmountMinor        string //
	CreatedAt             string //
	AllocationNo          string // Immutable local allocation owning this source tax item
	BillingInvoiceItemNo  string // Opaque core billing item reference used only for bounded capacity accounting
}

// taxInvoiceItemColumns holds the columns for the table plugin_idc_compliance_tax_invoice_item.
var taxInvoiceItemColumns = TaxInvoiceItemColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	ItemNo:                "item_no",
	TaxInvoiceNo:          "tax_invoice_no",
	LineNo:                "line_no",
	ItemSnapshotRef:       "item_snapshot_ref",
	ItemSnapshotSha256:    "item_snapshot_sha256",
	TaxClassificationCode: "tax_classification_code",
	TaxRatePpm:            "tax_rate_ppm",
	GrossAmountMinor:      "gross_amount_minor",
	NetAmountMinor:        "net_amount_minor",
	TaxAmountMinor:        "tax_amount_minor",
	CreatedAt:             "created_at",
	AllocationNo:          "allocation_no",
	BillingInvoiceItemNo:  "billing_invoice_item_no",
}

// NewTaxInvoiceItemDao creates and returns a new DAO object for table data access.
func NewTaxInvoiceItemDao(handlers ...gdb.ModelHandler) *TaxInvoiceItemDao {
	return &TaxInvoiceItemDao{
		group:    "default",
		table:    "plugin_idc_compliance_tax_invoice_item",
		columns:  taxInvoiceItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaxInvoiceItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaxInvoiceItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaxInvoiceItemDao) Columns() TaxInvoiceItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaxInvoiceItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaxInvoiceItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaxInvoiceItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
