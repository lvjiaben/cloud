// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionConfigurationDao is the data access object for the table plugin_idc_core_catalog_product_version_configuration.
type CatalogProductVersionConfigurationDao struct {
	table    string                                    // table is the underlying table name of the DAO.
	group    string                                    // group is the database configuration group name of the current DAO.
	columns  CatalogProductVersionConfigurationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                        // handlers for customized model modification.
}

// CatalogProductVersionConfigurationColumns defines and stores column names for the table plugin_idc_core_catalog_product_version_configuration.
type CatalogProductVersionConfigurationColumns struct {
	Id                             string //
	TenantId                       string //
	VersionNo                      string //
	InterfaceInstanceNo            string //
	InterfaceConfigurationRevision string //
	ProductSchemaVersion           string //
	ProductDeclarationSnapshot     string //
	MerchantValueSnapshot          string //
	CustomerSchemaVersion          string //
	CustomerDeclarationSnapshot    string //
	DeclarationDigest              string //
	MerchantValueDigest            string //
	Version                        string //
	ChangedBy                      string //
	CreatedAt                      string //
	ChangedAt                      string //
}

// catalogProductVersionConfigurationColumns holds the columns for the table plugin_idc_core_catalog_product_version_configuration.
var catalogProductVersionConfigurationColumns = CatalogProductVersionConfigurationColumns{
	Id:                             "id",
	TenantId:                       "tenant_id",
	VersionNo:                      "version_no",
	InterfaceInstanceNo:            "interface_instance_no",
	InterfaceConfigurationRevision: "interface_configuration_revision",
	ProductSchemaVersion:           "product_schema_version",
	ProductDeclarationSnapshot:     "product_declaration_snapshot",
	MerchantValueSnapshot:          "merchant_value_snapshot",
	CustomerSchemaVersion:          "customer_schema_version",
	CustomerDeclarationSnapshot:    "customer_declaration_snapshot",
	DeclarationDigest:              "declaration_digest",
	MerchantValueDigest:            "merchant_value_digest",
	Version:                        "version",
	ChangedBy:                      "changed_by",
	CreatedAt:                      "created_at",
	ChangedAt:                      "changed_at",
}

// NewCatalogProductVersionConfigurationDao creates and returns a new DAO object for table data access.
func NewCatalogProductVersionConfigurationDao(handlers ...gdb.ModelHandler) *CatalogProductVersionConfigurationDao {
	return &CatalogProductVersionConfigurationDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_product_version_configuration",
		columns:  catalogProductVersionConfigurationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogProductVersionConfigurationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogProductVersionConfigurationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogProductVersionConfigurationDao) Columns() CatalogProductVersionConfigurationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogProductVersionConfigurationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogProductVersionConfigurationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogProductVersionConfigurationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
