// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceDao is the data access object for the table plugin_idc_compliance_tax_invoice.
type TaxInvoiceDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TaxInvoiceColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TaxInvoiceColumns defines and stores column names for the table plugin_idc_compliance_tax_invoice.
type TaxInvoiceColumns struct {
	Id                           string //
	TenantId                     string //
	TaxInvoiceNo                 string //
	RequestSha256                string //
	CustomerNo                   string //
	PrincipalNo                  string //
	SellerSnapshotRef            string // Private immutable seller snapshot for the installation merchant, never the software platform
	SellerSnapshotSha256         string //
	BuyerSnapshotRef             string //
	BuyerSnapshotSha256          string //
	InvoiceTypeCode              string // Customer requests use pending_confirmation; an authorized reviewer selects a supported type later
	StatusCode                   string //
	TaxCurrencyCode              string //
	GrossAmountMinor             string //
	NetAmountMinor               string //
	TaxAmountMinor               string //
	BillingCurrencyCode          string //
	ExchangeRateNumerator        string //
	ExchangeRateDenominator      string //
	ExchangeRateDate             string //
	ExchangeRateSourceCode       string //
	ProviderCode                 string //
	ExternalInvoiceRef           string //
	MaskedInvoiceNumber          string //
	InvoiceNumberLookupDigest    string // Keyed lookup digest for exact invoice-number reconciliation; raw values and unsalted public hashes are forbidden
	SubmittedAt                  string //
	IssuedAt                     string //
	DeliveredAt                  string //
	CanceledAt                   string //
	Version                      string //
	CreatedAt                    string //
	UpdatedAt                    string //
	DecisionReasonCode           string //
	DecisionInternalNote         string // Optional administrator-only note; customer projections must never expose it
	ReviewedByUserId             string //
	ReviewedAt                   string //
	ProviderNotIssuedConfirmedAt string // Authoritative positive evidence required before safely canceling a failed provider attempt
}

// taxInvoiceColumns holds the columns for the table plugin_idc_compliance_tax_invoice.
var taxInvoiceColumns = TaxInvoiceColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	TaxInvoiceNo:                 "tax_invoice_no",
	RequestSha256:                "request_sha256",
	CustomerNo:                   "customer_no",
	PrincipalNo:                  "principal_no",
	SellerSnapshotRef:            "seller_snapshot_ref",
	SellerSnapshotSha256:         "seller_snapshot_sha256",
	BuyerSnapshotRef:             "buyer_snapshot_ref",
	BuyerSnapshotSha256:          "buyer_snapshot_sha256",
	InvoiceTypeCode:              "invoice_type_code",
	StatusCode:                   "status_code",
	TaxCurrencyCode:              "tax_currency_code",
	GrossAmountMinor:             "gross_amount_minor",
	NetAmountMinor:               "net_amount_minor",
	TaxAmountMinor:               "tax_amount_minor",
	BillingCurrencyCode:          "billing_currency_code",
	ExchangeRateNumerator:        "exchange_rate_numerator",
	ExchangeRateDenominator:      "exchange_rate_denominator",
	ExchangeRateDate:             "exchange_rate_date",
	ExchangeRateSourceCode:       "exchange_rate_source_code",
	ProviderCode:                 "provider_code",
	ExternalInvoiceRef:           "external_invoice_ref",
	MaskedInvoiceNumber:          "masked_invoice_number",
	InvoiceNumberLookupDigest:    "invoice_number_lookup_digest",
	SubmittedAt:                  "submitted_at",
	IssuedAt:                     "issued_at",
	DeliveredAt:                  "delivered_at",
	CanceledAt:                   "canceled_at",
	Version:                      "version",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
	DecisionReasonCode:           "decision_reason_code",
	DecisionInternalNote:         "decision_internal_note",
	ReviewedByUserId:             "reviewed_by_user_id",
	ReviewedAt:                   "reviewed_at",
	ProviderNotIssuedConfirmedAt: "provider_not_issued_confirmed_at",
}

// NewTaxInvoiceDao creates and returns a new DAO object for table data access.
func NewTaxInvoiceDao(handlers ...gdb.ModelHandler) *TaxInvoiceDao {
	return &TaxInvoiceDao{
		group:    "default",
		table:    "plugin_idc_compliance_tax_invoice",
		columns:  taxInvoiceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaxInvoiceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaxInvoiceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaxInvoiceDao) Columns() TaxInvoiceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaxInvoiceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaxInvoiceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaxInvoiceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
