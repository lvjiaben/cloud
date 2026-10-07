// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogResourcePoolBindingDao is the data access object for the table plugin_idc_core_catalog_resource_pool_binding.
type CatalogResourcePoolBindingDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  CatalogResourcePoolBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// CatalogResourcePoolBindingColumns defines and stores column names for the table plugin_idc_core_catalog_resource_pool_binding.
type CatalogResourcePoolBindingColumns struct {
	Id                      string //
	TenantId                string //
	BindingNo               string //
	VersionNo               string //
	PoolNo                  string // Opaque resource-owner pool business number; no cross-plugin foreign key
	RegionCode              string //
	LineCode                string //
	Priority                string //
	Weight                  string //
	NewOrderEnabled         string //
	StatusCode              string //
	CreatedAt               string //
	MaintenanceModeCode     string // Normal, drain, or maintenance mode for new-order routing only
	MinimumSellableQuantity string // Local sellable quantity threshold required before automatic fulfillment is promised
	UncertainSalePolicyCode string // Fail-closed stop-sale or explicit manual-confirmation policy for uncertain inventory
}

// catalogResourcePoolBindingColumns holds the columns for the table plugin_idc_core_catalog_resource_pool_binding.
var catalogResourcePoolBindingColumns = CatalogResourcePoolBindingColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	BindingNo:               "binding_no",
	VersionNo:               "version_no",
	PoolNo:                  "pool_no",
	RegionCode:              "region_code",
	LineCode:                "line_code",
	Priority:                "priority",
	Weight:                  "weight",
	NewOrderEnabled:         "new_order_enabled",
	StatusCode:              "status_code",
	CreatedAt:               "created_at",
	MaintenanceModeCode:     "maintenance_mode_code",
	MinimumSellableQuantity: "minimum_sellable_quantity",
	UncertainSalePolicyCode: "uncertain_sale_policy_code",
}

// NewCatalogResourcePoolBindingDao creates and returns a new DAO object for table data access.
func NewCatalogResourcePoolBindingDao(handlers ...gdb.ModelHandler) *CatalogResourcePoolBindingDao {
	return &CatalogResourcePoolBindingDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_resource_pool_binding",
		columns:  catalogResourcePoolBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogResourcePoolBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogResourcePoolBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogResourcePoolBindingDao) Columns() CatalogResourcePoolBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogResourcePoolBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogResourcePoolBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogResourcePoolBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
