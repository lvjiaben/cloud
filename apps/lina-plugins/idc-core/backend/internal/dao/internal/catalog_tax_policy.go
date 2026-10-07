// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogTaxPolicyDao is the data access object for the table plugin_idc_core_catalog_tax_policy.
type CatalogTaxPolicyDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  CatalogTaxPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// CatalogTaxPolicyColumns defines and stores column names for the table plugin_idc_core_catalog_tax_policy.
type CatalogTaxPolicyColumns struct {
	Id                   string //
	TenantId             string //
	TaxPolicyNo          string //
	VersionNo            string //
	CurrencyCode         string //
	PriceIncludesTax     string //
	TaxTreatmentCode     string //
	TaxRateBasisPoints   string // Tax rate in basis points; null only for exempt treatment
	TaxCategoryCode      string //
	RoundingScopeCode    string //
	RoundingModeCode     string //
	RoundingQuantumMinor string // Positive rounding quantum expressed in the configured currency minor unit
	CreatedAt            string //
}

// catalogTaxPolicyColumns holds the columns for the table plugin_idc_core_catalog_tax_policy.
var catalogTaxPolicyColumns = CatalogTaxPolicyColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	TaxPolicyNo:          "tax_policy_no",
	VersionNo:            "version_no",
	CurrencyCode:         "currency_code",
	PriceIncludesTax:     "price_includes_tax",
	TaxTreatmentCode:     "tax_treatment_code",
	TaxRateBasisPoints:   "tax_rate_basis_points",
	TaxCategoryCode:      "tax_category_code",
	RoundingScopeCode:    "rounding_scope_code",
	RoundingModeCode:     "rounding_mode_code",
	RoundingQuantumMinor: "rounding_quantum_minor",
	CreatedAt:            "created_at",
}

// NewCatalogTaxPolicyDao creates and returns a new DAO object for table data access.
func NewCatalogTaxPolicyDao(handlers ...gdb.ModelHandler) *CatalogTaxPolicyDao {
	return &CatalogTaxPolicyDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_tax_policy",
		columns:  catalogTaxPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogTaxPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogTaxPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogTaxPolicyDao) Columns() CatalogTaxPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogTaxPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogTaxPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogTaxPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
