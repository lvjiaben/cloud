// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingFundHoldDao is the data access object for the table plugin_idc_core_billing_fund_hold.
type BillingFundHoldDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  BillingFundHoldColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// BillingFundHoldColumns defines and stores column names for the table plugin_idc_core_billing_fund_hold.
type BillingFundHoldColumns struct {
	Id                  string //
	TenantId            string //
	HoldNo              string //
	CustomerNo          string //
	AccountNo           string //
	AccountTypeCode     string //
	CurrencyCode        string //
	SourceTypeCode      string //
	SourceReference     string //
	AmountMinor         string //
	ConsumedAmountMinor string //
	ReleasedAmountMinor string //
	StatusCode          string //
	ContentDigest       string //
	ExpiresAt           string //
	ConsumedAt          string //
	ReleasedAt          string //
	Version             string //
	CreatedAt           string //
	UpdatedAt           string //
}

// billingFundHoldColumns holds the columns for the table plugin_idc_core_billing_fund_hold.
var billingFundHoldColumns = BillingFundHoldColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	HoldNo:              "hold_no",
	CustomerNo:          "customer_no",
	AccountNo:           "account_no",
	AccountTypeCode:     "account_type_code",
	CurrencyCode:        "currency_code",
	SourceTypeCode:      "source_type_code",
	SourceReference:     "source_reference",
	AmountMinor:         "amount_minor",
	ConsumedAmountMinor: "consumed_amount_minor",
	ReleasedAmountMinor: "released_amount_minor",
	StatusCode:          "status_code",
	ContentDigest:       "content_digest",
	ExpiresAt:           "expires_at",
	ConsumedAt:          "consumed_at",
	ReleasedAt:          "released_at",
	Version:             "version",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewBillingFundHoldDao creates and returns a new DAO object for table data access.
func NewBillingFundHoldDao(handlers ...gdb.ModelHandler) *BillingFundHoldDao {
	return &BillingFundHoldDao{
		group:    "default",
		table:    "plugin_idc_core_billing_fund_hold",
		columns:  billingFundHoldColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingFundHoldDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingFundHoldDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingFundHoldDao) Columns() BillingFundHoldColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingFundHoldDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingFundHoldDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingFundHoldDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
