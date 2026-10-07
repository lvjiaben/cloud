// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceFileDao is the data access object for the table plugin_idc_compliance_tax_invoice_file.
type TaxInvoiceFileDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  TaxInvoiceFileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// TaxInvoiceFileColumns defines and stores column names for the table plugin_idc_compliance_tax_invoice_file.
type TaxInvoiceFileColumns struct {
	Id               string //
	TenantId         string //
	FileNo           string //
	TaxInvoiceNo     string //
	FileTypeCode     string //
	PrivateObjectRef string //
	ContentSha256    string //
	MediaType        string //
	ByteSize         string //
	StatusCode       string //
	Immutable        string //
	ArchivedAt       string //
	RetentionUntil   string //
	LegalHold        string //
	CreatedAt        string //
	UpdatedAt        string //
}

// taxInvoiceFileColumns holds the columns for the table plugin_idc_compliance_tax_invoice_file.
var taxInvoiceFileColumns = TaxInvoiceFileColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	FileNo:           "file_no",
	TaxInvoiceNo:     "tax_invoice_no",
	FileTypeCode:     "file_type_code",
	PrivateObjectRef: "private_object_ref",
	ContentSha256:    "content_sha256",
	MediaType:        "media_type",
	ByteSize:         "byte_size",
	StatusCode:       "status_code",
	Immutable:        "immutable",
	ArchivedAt:       "archived_at",
	RetentionUntil:   "retention_until",
	LegalHold:        "legal_hold",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewTaxInvoiceFileDao creates and returns a new DAO object for table data access.
func NewTaxInvoiceFileDao(handlers ...gdb.ModelHandler) *TaxInvoiceFileDao {
	return &TaxInvoiceFileDao{
		group:    "default",
		table:    "plugin_idc_compliance_tax_invoice_file",
		columns:  taxInvoiceFileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaxInvoiceFileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaxInvoiceFileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaxInvoiceFileDao) Columns() TaxInvoiceFileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaxInvoiceFileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaxInvoiceFileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaxInvoiceFileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
