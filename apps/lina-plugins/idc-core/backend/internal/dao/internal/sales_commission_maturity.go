// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionMaturityDao is the data access object for the table plugin_idc_core_sales_commission_maturity.
type SalesCommissionMaturityDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SalesCommissionMaturityColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SalesCommissionMaturityColumns defines and stores column names for the table plugin_idc_core_sales_commission_maturity.
type SalesCommissionMaturityColumns struct {
	Id                   string //
	TenantId             string //
	MaturityNo           string //
	CommissionNo         string //
	AttemptNo            string //
	AccountNo            string //
	CurrencyCode         string //
	BeneficiaryTypeCode  string //
	BeneficiaryReference string //
	AmountMinor          string //
	StatusCode           string //
	ReasonCode           string //
	EvidenceDigest       string //
	ProcessedAt          string //
}

// salesCommissionMaturityColumns holds the columns for the table plugin_idc_core_sales_commission_maturity.
var salesCommissionMaturityColumns = SalesCommissionMaturityColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	MaturityNo:           "maturity_no",
	CommissionNo:         "commission_no",
	AttemptNo:            "attempt_no",
	AccountNo:            "account_no",
	CurrencyCode:         "currency_code",
	BeneficiaryTypeCode:  "beneficiary_type_code",
	BeneficiaryReference: "beneficiary_reference",
	AmountMinor:          "amount_minor",
	StatusCode:           "status_code",
	ReasonCode:           "reason_code",
	EvidenceDigest:       "evidence_digest",
	ProcessedAt:          "processed_at",
}

// NewSalesCommissionMaturityDao creates and returns a new DAO object for table data access.
func NewSalesCommissionMaturityDao(handlers ...gdb.ModelHandler) *SalesCommissionMaturityDao {
	return &SalesCommissionMaturityDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_maturity",
		columns:  salesCommissionMaturityColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionMaturityDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionMaturityDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionMaturityDao) Columns() SalesCommissionMaturityColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionMaturityDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionMaturityDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionMaturityDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
