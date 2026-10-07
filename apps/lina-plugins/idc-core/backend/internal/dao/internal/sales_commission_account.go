// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionAccountDao is the data access object for the table plugin_idc_core_sales_commission_account.
type SalesCommissionAccountDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  SalesCommissionAccountColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// SalesCommissionAccountColumns defines and stores column names for the table plugin_idc_core_sales_commission_account.
type SalesCommissionAccountColumns struct {
	Id                   string //
	TenantId             string //
	AccountNo            string //
	BeneficiaryTypeCode  string //
	BeneficiaryReference string //
	CurrencyCode         string //
	CreditedAmountMinor  string //
	PendingAmountMinor   string //
	AvailableAmountMinor string //
	FrozenAmountMinor    string //
	PaidAmountMinor      string //
	ReversedAmountMinor  string //
	StatusCode           string //
	Version              string //
	CreatedAt            string //
	UpdatedAt            string //
}

// salesCommissionAccountColumns holds the columns for the table plugin_idc_core_sales_commission_account.
var salesCommissionAccountColumns = SalesCommissionAccountColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	AccountNo:            "account_no",
	BeneficiaryTypeCode:  "beneficiary_type_code",
	BeneficiaryReference: "beneficiary_reference",
	CurrencyCode:         "currency_code",
	CreditedAmountMinor:  "credited_amount_minor",
	PendingAmountMinor:   "pending_amount_minor",
	AvailableAmountMinor: "available_amount_minor",
	FrozenAmountMinor:    "frozen_amount_minor",
	PaidAmountMinor:      "paid_amount_minor",
	ReversedAmountMinor:  "reversed_amount_minor",
	StatusCode:           "status_code",
	Version:              "version",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewSalesCommissionAccountDao creates and returns a new DAO object for table data access.
func NewSalesCommissionAccountDao(handlers ...gdb.ModelHandler) *SalesCommissionAccountDao {
	return &SalesCommissionAccountDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_account",
		columns:  salesCommissionAccountColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionAccountDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionAccountDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionAccountDao) Columns() SalesCommissionAccountColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionAccountDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionAccountDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionAccountDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
