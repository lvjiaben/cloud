// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogConfigurationOptionDao is the data access object for the table plugin_idc_core_catalog_configuration_option.
type CatalogConfigurationOptionDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  CatalogConfigurationOptionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// CatalogConfigurationOptionColumns defines and stores column names for the table plugin_idc_core_catalog_configuration_option.
type CatalogConfigurationOptionColumns struct {
	Id                string //
	TenantId          string //
	OptionNo          string //
	GroupNo           string //
	VersionNo         string //
	OptionCode        string //
	ComponentTypeCode string //
	NameZhCn          string //
	NameEnUs          string //
	NormalizedValue   string // Business-normalized value object; raw Provider responses and credentials are forbidden
	MinimumQuantity   string //
	MaximumQuantity   string //
	DefaultQuantity   string //
	SortOrder         string //
	StatusCode        string //
	CreatedAt         string //
}

// catalogConfigurationOptionColumns holds the columns for the table plugin_idc_core_catalog_configuration_option.
var catalogConfigurationOptionColumns = CatalogConfigurationOptionColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	OptionNo:          "option_no",
	GroupNo:           "group_no",
	VersionNo:         "version_no",
	OptionCode:        "option_code",
	ComponentTypeCode: "component_type_code",
	NameZhCn:          "name_zh_cn",
	NameEnUs:          "name_en_us",
	NormalizedValue:   "normalized_value",
	MinimumQuantity:   "minimum_quantity",
	MaximumQuantity:   "maximum_quantity",
	DefaultQuantity:   "default_quantity",
	SortOrder:         "sort_order",
	StatusCode:        "status_code",
	CreatedAt:         "created_at",
}

// NewCatalogConfigurationOptionDao creates and returns a new DAO object for table data access.
func NewCatalogConfigurationOptionDao(handlers ...gdb.ModelHandler) *CatalogConfigurationOptionDao {
	return &CatalogConfigurationOptionDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_configuration_option",
		columns:  catalogConfigurationOptionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogConfigurationOptionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogConfigurationOptionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogConfigurationOptionDao) Columns() CatalogConfigurationOptionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogConfigurationOptionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogConfigurationOptionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogConfigurationOptionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
