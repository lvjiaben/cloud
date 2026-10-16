// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesPurchasePriceDao is the data access object for the table plugin_idc_core_sales_purchase_price.
type SalesPurchasePriceDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  SalesPurchasePriceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// SalesPurchasePriceColumns defines and stores column names for the table plugin_idc_core_sales_purchase_price.
type SalesPurchasePriceColumns struct {
	Id                  string //
	TenantId            string //
	PriceNo             string //
	CustomerNo          string //
	LevelNo             string //
	ProductNo           string //
	CurrencyCode        string //
	ChargeTypeCode      string //
	PeriodUnitCode      string //
	PeriodCount         string //
	AdjustmentTypeCode  string //
	FixedPriceMinor     string //
	DiscountBasisPoints string //
	StatusCode          string //
	Version             string //
	ContentDigest       string //
	CreatedAt           string //
}

// salesPurchasePriceColumns holds the columns for the table plugin_idc_core_sales_purchase_price.
var salesPurchasePriceColumns = SalesPurchasePriceColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	PriceNo:             "price_no",
	CustomerNo:          "customer_no",
	LevelNo:             "level_no",
	ProductNo:           "product_no",
	CurrencyCode:        "currency_code",
	ChargeTypeCode:      "charge_type_code",
	PeriodUnitCode:      "period_unit_code",
	PeriodCount:         "period_count",
	AdjustmentTypeCode:  "adjustment_type_code",
	FixedPriceMinor:     "fixed_price_minor",
	DiscountBasisPoints: "discount_basis_points",
	StatusCode:          "status_code",
	Version:             "version",
	ContentDigest:       "content_digest",
	CreatedAt:           "created_at",
}

// NewSalesPurchasePriceDao creates and returns a new DAO object for table data access.
func NewSalesPurchasePriceDao(handlers ...gdb.ModelHandler) *SalesPurchasePriceDao {
	return &SalesPurchasePriceDao{
		group:    "default",
		table:    "plugin_idc_core_sales_purchase_price",
		columns:  salesPurchasePriceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesPurchasePriceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesPurchasePriceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesPurchasePriceDao) Columns() SalesPurchasePriceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesPurchasePriceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesPurchasePriceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesPurchasePriceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
