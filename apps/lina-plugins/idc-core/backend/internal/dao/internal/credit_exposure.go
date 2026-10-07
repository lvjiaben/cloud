// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditExposureDao is the data access object for the table plugin_idc_core_credit_exposure.
type CreditExposureDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  CreditExposureColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// CreditExposureColumns defines and stores column names for the table plugin_idc_core_credit_exposure.
type CreditExposureColumns struct {
	Id                       string //
	TenantId                 string //
	ExposureNo               string //
	CreditAccountNo          string //
	CustomerNo               string //
	CurrencyCode             string //
	SourceTypeCode           string //
	SourceReference          string //
	OrderNo                  string //
	InvoiceNo                string //
	OriginalPrincipalMinor   string //
	UnbilledPrincipalMinor   string //
	BilledPrincipalMinor     string //
	RepaidPrincipalMinor     string //
	ReleasedPrincipalMinor   string //
	WrittenOffPrincipalMinor string //
	StatusCode               string //
	SourceSnapshotDigest     string //
	ContentDigest            string //
	Version                  string //
	CreatedAt                string //
	UpdatedAt                string //
}

// creditExposureColumns holds the columns for the table plugin_idc_core_credit_exposure.
var creditExposureColumns = CreditExposureColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	ExposureNo:               "exposure_no",
	CreditAccountNo:          "credit_account_no",
	CustomerNo:               "customer_no",
	CurrencyCode:             "currency_code",
	SourceTypeCode:           "source_type_code",
	SourceReference:          "source_reference",
	OrderNo:                  "order_no",
	InvoiceNo:                "invoice_no",
	OriginalPrincipalMinor:   "original_principal_minor",
	UnbilledPrincipalMinor:   "unbilled_principal_minor",
	BilledPrincipalMinor:     "billed_principal_minor",
	RepaidPrincipalMinor:     "repaid_principal_minor",
	ReleasedPrincipalMinor:   "released_principal_minor",
	WrittenOffPrincipalMinor: "written_off_principal_minor",
	StatusCode:               "status_code",
	SourceSnapshotDigest:     "source_snapshot_digest",
	ContentDigest:            "content_digest",
	Version:                  "version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewCreditExposureDao creates and returns a new DAO object for table data access.
func NewCreditExposureDao(handlers ...gdb.ModelHandler) *CreditExposureDao {
	return &CreditExposureDao{
		group:    "default",
		table:    "plugin_idc_core_credit_exposure",
		columns:  creditExposureColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditExposureDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditExposureDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditExposureDao) Columns() CreditExposureColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditExposureDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditExposureDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditExposureDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
