// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionFulfillmentBindingDao is the data access object for the table plugin_idc_core_catalog_product_version_fulfillment_binding.
type CatalogProductVersionFulfillmentBindingDao struct {
	table    string                                         // table is the underlying table name of the DAO.
	group    string                                         // group is the database configuration group name of the current DAO.
	columns  CatalogProductVersionFulfillmentBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                             // handlers for customized model modification.
}

// CatalogProductVersionFulfillmentBindingColumns defines and stores column names for the table plugin_idc_core_catalog_product_version_fulfillment_binding.
type CatalogProductVersionFulfillmentBindingColumns struct {
	Id                           string //
	TenantId                     string //
	VersionNo                    string //
	InterfaceInstanceNo          string //
	InterfaceDisplayNameSnapshot string //
	PluginId                     string //
	ExtensionPointCode           string //
	PluginReleaseVersion         string //
	ContractVersion              string //
	AuthorizedMethods            string //
	ReleaseDigest                string //
	DeclarationDigest            string //
	BoundBy                      string //
	BoundAt                      string //
}

// catalogProductVersionFulfillmentBindingColumns holds the columns for the table plugin_idc_core_catalog_product_version_fulfillment_binding.
var catalogProductVersionFulfillmentBindingColumns = CatalogProductVersionFulfillmentBindingColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	VersionNo:                    "version_no",
	InterfaceInstanceNo:          "interface_instance_no",
	InterfaceDisplayNameSnapshot: "interface_display_name_snapshot",
	PluginId:                     "plugin_id",
	ExtensionPointCode:           "extension_point_code",
	PluginReleaseVersion:         "plugin_release_version",
	ContractVersion:              "contract_version",
	AuthorizedMethods:            "authorized_methods",
	ReleaseDigest:                "release_digest",
	DeclarationDigest:            "declaration_digest",
	BoundBy:                      "bound_by",
	BoundAt:                      "bound_at",
}

// NewCatalogProductVersionFulfillmentBindingDao creates and returns a new DAO object for table data access.
func NewCatalogProductVersionFulfillmentBindingDao(handlers ...gdb.ModelHandler) *CatalogProductVersionFulfillmentBindingDao {
	return &CatalogProductVersionFulfillmentBindingDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_product_version_fulfillment_binding",
		columns:  catalogProductVersionFulfillmentBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogProductVersionFulfillmentBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogProductVersionFulfillmentBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogProductVersionFulfillmentBindingDao) Columns() CatalogProductVersionFulfillmentBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogProductVersionFulfillmentBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogProductVersionFulfillmentBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogProductVersionFulfillmentBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
