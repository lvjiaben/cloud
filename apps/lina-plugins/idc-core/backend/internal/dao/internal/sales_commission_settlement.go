// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionSettlementDao is the data access object for the table plugin_idc_core_sales_commission_settlement.
type SalesCommissionSettlementDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  SalesCommissionSettlementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// SalesCommissionSettlementColumns defines and stores column names for the table plugin_idc_core_sales_commission_settlement.
type SalesCommissionSettlementColumns struct {
	Id                   string //
	TenantId             string //
	SettlementNo         string //
	AccountNo            string //
	CurrencyCode         string //
	PayeeProfileNo       string //
	BeneficiaryTypeCode  string //
	BeneficiaryReference string //
	RequestedAmountMinor string //
	FrozenAmountMinor    string //
	PaidAmountMinor      string //
	ReturnedAmountMinor  string //
	MinimumAmountMinor   string //
	StatusCode           string //
	RequestDigest        string //
	RequestedAt          string //
	CompletedAt          string //
	Version              string //
}

// salesCommissionSettlementColumns holds the columns for the table plugin_idc_core_sales_commission_settlement.
var salesCommissionSettlementColumns = SalesCommissionSettlementColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	SettlementNo:         "settlement_no",
	AccountNo:            "account_no",
	CurrencyCode:         "currency_code",
	PayeeProfileNo:       "payee_profile_no",
	BeneficiaryTypeCode:  "beneficiary_type_code",
	BeneficiaryReference: "beneficiary_reference",
	RequestedAmountMinor: "requested_amount_minor",
	FrozenAmountMinor:    "frozen_amount_minor",
	PaidAmountMinor:      "paid_amount_minor",
	ReturnedAmountMinor:  "returned_amount_minor",
	MinimumAmountMinor:   "minimum_amount_minor",
	StatusCode:           "status_code",
	RequestDigest:        "request_digest",
	RequestedAt:          "requested_at",
	CompletedAt:          "completed_at",
	Version:              "version",
}

// NewSalesCommissionSettlementDao creates and returns a new DAO object for table data access.
func NewSalesCommissionSettlementDao(handlers ...gdb.ModelHandler) *SalesCommissionSettlementDao {
	return &SalesCommissionSettlementDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_settlement",
		columns:  salesCommissionSettlementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionSettlementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionSettlementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionSettlementDao) Columns() SalesCommissionSettlementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionSettlementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionSettlementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionSettlementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
