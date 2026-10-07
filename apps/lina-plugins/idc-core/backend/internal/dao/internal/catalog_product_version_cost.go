// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionCostDao is the data access object for the table plugin_idc_core_catalog_product_version_cost.
type CatalogProductVersionCostDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CatalogProductVersionCostColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CatalogProductVersionCostColumns defines and stores column names for the table plugin_idc_core_catalog_product_version_cost.
type CatalogProductVersionCostColumns struct {
	Id             string //
	TenantId       string //
	CostNo         string //
	VersionNo      string //
	CurrencyCode   string //
	PeriodUnitCode string //
	PeriodCount    string //
	UnitCostMinor  string //
	EffectiveFrom  string //
	EffectiveUntil string //
	Version        string //
	CreatedAt      string //
	UpdatedAt      string //
}

// catalogProductVersionCostColumns holds the columns for the table plugin_idc_core_catalog_product_version_cost.
var catalogProductVersionCostColumns = CatalogProductVersionCostColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	CostNo:         "cost_no",
	VersionNo:      "version_no",
	CurrencyCode:   "currency_code",
	PeriodUnitCode: "period_unit_code",
	PeriodCount:    "period_count",
	UnitCostMinor:  "unit_cost_minor",
	EffectiveFrom:  "effective_from",
	EffectiveUntil: "effective_until",
	Version:        "version",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewCatalogProductVersionCostDao creates and returns a new DAO object for table data access.
func NewCatalogProductVersionCostDao(handlers ...gdb.ModelHandler) *CatalogProductVersionCostDao {
	return &CatalogProductVersionCostDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_product_version_cost",
		columns:  catalogProductVersionCostColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogProductVersionCostDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogProductVersionCostDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogProductVersionCostDao) Columns() CatalogProductVersionCostColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogProductVersionCostDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogProductVersionCostDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogProductVersionCostDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
