// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MerchantCurrencyDao is the data access object for the table plugin_idc_core_merchant_currency.
type MerchantCurrencyDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  MerchantCurrencyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// MerchantCurrencyColumns defines and stores column names for the table plugin_idc_core_merchant_currency.
type MerchantCurrencyColumns struct {
	TenantId     string //
	CurrencyCode string // ISO 4217 currency code
	Enabled      string // Whether new quotes may use this currency
	Version      string // Optimistic concurrency version
	CreatedBy    string //
	UpdatedBy    string //
	CreatedAt    string //
	UpdatedAt    string //
	Deleted      string // Whether the configuration is removed; retain version and audit continuity
}

// merchantCurrencyColumns holds the columns for the table plugin_idc_core_merchant_currency.
var merchantCurrencyColumns = MerchantCurrencyColumns{
	TenantId:     "tenant_id",
	CurrencyCode: "currency_code",
	Enabled:      "enabled",
	Version:      "version",
	CreatedBy:    "created_by",
	UpdatedBy:    "updated_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
	Deleted:      "deleted",
}

// NewMerchantCurrencyDao creates and returns a new DAO object for table data access.
func NewMerchantCurrencyDao(handlers ...gdb.ModelHandler) *MerchantCurrencyDao {
	return &MerchantCurrencyDao{
		group:    "default",
		table:    "plugin_idc_core_merchant_currency",
		columns:  merchantCurrencyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MerchantCurrencyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MerchantCurrencyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MerchantCurrencyDao) Columns() MerchantCurrencyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MerchantCurrencyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MerchantCurrencyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MerchantCurrencyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
