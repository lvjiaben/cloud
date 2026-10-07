// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductImportDao is the data access object for the table plugin_idc_core_catalog_product_import.
type CatalogProductImportDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  CatalogProductImportColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// CatalogProductImportColumns defines and stores column names for the table plugin_idc_core_catalog_product_import.
type CatalogProductImportColumns struct {
	Id                           string //
	TenantId                     string //
	RequestKey                   string //
	RequestDigest                string //
	ActorUserId                  string //
	InterfaceInstanceNo          string //
	UpstreamReference            string //
	ProductNo                    string //
	VersionNo                    string //
	SourceSnapshot               string //
	MissingConfigurationSnapshot string //
	PricingIncomplete            string //
	CreatedAt                    string //
}

// catalogProductImportColumns holds the columns for the table plugin_idc_core_catalog_product_import.
var catalogProductImportColumns = CatalogProductImportColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	RequestKey:                   "request_key",
	RequestDigest:                "request_digest",
	ActorUserId:                  "actor_user_id",
	InterfaceInstanceNo:          "interface_instance_no",
	UpstreamReference:            "upstream_reference",
	ProductNo:                    "product_no",
	VersionNo:                    "version_no",
	SourceSnapshot:               "source_snapshot",
	MissingConfigurationSnapshot: "missing_configuration_snapshot",
	PricingIncomplete:            "pricing_incomplete",
	CreatedAt:                    "created_at",
}

// NewCatalogProductImportDao creates and returns a new DAO object for table data access.
func NewCatalogProductImportDao(handlers ...gdb.ModelHandler) *CatalogProductImportDao {
	return &CatalogProductImportDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_product_import",
		columns:  catalogProductImportColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogProductImportDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogProductImportDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogProductImportDao) Columns() CatalogProductImportColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogProductImportDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogProductImportDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogProductImportDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
