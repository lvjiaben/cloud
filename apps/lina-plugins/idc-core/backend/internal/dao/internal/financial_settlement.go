// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSettlementDao is the data access object for the table plugin_idc_core_financial_settlement.
type FinancialSettlementDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  FinancialSettlementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// FinancialSettlementColumns defines and stores column names for the table plugin_idc_core_financial_settlement.
type FinancialSettlementColumns struct {
	Id                     string //
	TenantId               string //
	SettlementNo           string //
	OrderNo                string //
	InvoiceNo              string //
	CustomerNo             string //
	CurrencyCode           string //
	BasisCode              string // Mutually exclusive funded, credit, zero-amount, full-discount, or trial basis
	TotalAmountMinor       string //
	PaidAmountMinor        string //
	CreditAmountMinor      string //
	SourceCount            string //
	IdentityEvidenceDigest string // Digest of the re-read customer identity authority without personal material
	SourceSnapshotDigest   string //
	ContentDigest          string //
	SettledAt              string //
	CreatedAt              string //
}

// financialSettlementColumns holds the columns for the table plugin_idc_core_financial_settlement.
var financialSettlementColumns = FinancialSettlementColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	SettlementNo:           "settlement_no",
	OrderNo:                "order_no",
	InvoiceNo:              "invoice_no",
	CustomerNo:             "customer_no",
	CurrencyCode:           "currency_code",
	BasisCode:              "basis_code",
	TotalAmountMinor:       "total_amount_minor",
	PaidAmountMinor:        "paid_amount_minor",
	CreditAmountMinor:      "credit_amount_minor",
	SourceCount:            "source_count",
	IdentityEvidenceDigest: "identity_evidence_digest",
	SourceSnapshotDigest:   "source_snapshot_digest",
	ContentDigest:          "content_digest",
	SettledAt:              "settled_at",
	CreatedAt:              "created_at",
}

// NewFinancialSettlementDao creates and returns a new DAO object for table data access.
func NewFinancialSettlementDao(handlers ...gdb.ModelHandler) *FinancialSettlementDao {
	return &FinancialSettlementDao{
		group:    "default",
		table:    "plugin_idc_core_financial_settlement",
		columns:  financialSettlementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialSettlementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialSettlementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialSettlementDao) Columns() FinancialSettlementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialSettlementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialSettlementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialSettlementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
