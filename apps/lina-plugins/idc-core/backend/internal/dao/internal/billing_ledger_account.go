// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingLedgerAccountDao is the data access object for the table plugin_idc_core_billing_ledger_account.
type BillingLedgerAccountDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  BillingLedgerAccountColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// BillingLedgerAccountColumns defines and stores column names for the table plugin_idc_core_billing_ledger_account.
type BillingLedgerAccountColumns struct {
	Id                string //
	TenantId          string //
	AccountNo         string //
	OwnerTypeCode     string //
	OwnerReference    string //
	CustomerNo        string //
	AccountTypeCode   string //
	NormalBalanceCode string //
	CurrencyCode      string //
	StatusCode        string //
	Withdrawable      string //
	ClosedAt          string //
	Version           string //
	CreatedAt         string //
	UpdatedAt         string //
}

// billingLedgerAccountColumns holds the columns for the table plugin_idc_core_billing_ledger_account.
var billingLedgerAccountColumns = BillingLedgerAccountColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	AccountNo:         "account_no",
	OwnerTypeCode:     "owner_type_code",
	OwnerReference:    "owner_reference",
	CustomerNo:        "customer_no",
	AccountTypeCode:   "account_type_code",
	NormalBalanceCode: "normal_balance_code",
	CurrencyCode:      "currency_code",
	StatusCode:        "status_code",
	Withdrawable:      "withdrawable",
	ClosedAt:          "closed_at",
	Version:           "version",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewBillingLedgerAccountDao creates and returns a new DAO object for table data access.
func NewBillingLedgerAccountDao(handlers ...gdb.ModelHandler) *BillingLedgerAccountDao {
	return &BillingLedgerAccountDao{
		group:    "default",
		table:    "plugin_idc_core_billing_ledger_account",
		columns:  billingLedgerAccountColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingLedgerAccountDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingLedgerAccountDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingLedgerAccountDao) Columns() BillingLedgerAccountColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingLedgerAccountDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingLedgerAccountDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingLedgerAccountDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
