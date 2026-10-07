// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogPriceDao is the data access object for the table plugin_idc_core_catalog_price.
type CatalogPriceDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  CatalogPriceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// CatalogPriceColumns defines and stores column names for the table plugin_idc_core_catalog_price.
type CatalogPriceColumns struct {
	Id                            string //
	TenantId                      string //
	PriceNo                       string //
	VersionNo                     string //
	BillingRuleNo                 string //
	OptionNo                      string // Null for the product base price, otherwise a version-owned option business number
	CurrencyCode                  string //
	AmountMinor                   string //
	SetupFeeMinor                 string //
	RenewalAmountMinor            string //
	MinimumQuantity               string //
	MaximumQuantity               string //
	StatusCode                    string //
	CreatedAt                     string //
	ConfigurationPriceDimensionNo string //
}

// catalogPriceColumns holds the columns for the table plugin_idc_core_catalog_price.
var catalogPriceColumns = CatalogPriceColumns{
	Id:                            "id",
	TenantId:                      "tenant_id",
	PriceNo:                       "price_no",
	VersionNo:                     "version_no",
	BillingRuleNo:                 "billing_rule_no",
	OptionNo:                      "option_no",
	CurrencyCode:                  "currency_code",
	AmountMinor:                   "amount_minor",
	SetupFeeMinor:                 "setup_fee_minor",
	RenewalAmountMinor:            "renewal_amount_minor",
	MinimumQuantity:               "minimum_quantity",
	MaximumQuantity:               "maximum_quantity",
	StatusCode:                    "status_code",
	CreatedAt:                     "created_at",
	ConfigurationPriceDimensionNo: "configuration_price_dimension_no",
}

// NewCatalogPriceDao creates and returns a new DAO object for table data access.
func NewCatalogPriceDao(handlers ...gdb.ModelHandler) *CatalogPriceDao {
	return &CatalogPriceDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_price",
		columns:  catalogPriceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogPriceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogPriceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogPriceDao) Columns() CatalogPriceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogPriceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogPriceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogPriceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
