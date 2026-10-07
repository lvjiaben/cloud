// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogConfigurationGroupDao is the data access object for the table plugin_idc_core_catalog_configuration_group.
type CatalogConfigurationGroupDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CatalogConfigurationGroupColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CatalogConfigurationGroupColumns defines and stores column names for the table plugin_idc_core_catalog_configuration_group.
type CatalogConfigurationGroupColumns struct {
	Id                string //
	TenantId          string //
	GroupNo           string //
	VersionNo         string //
	GroupCode         string //
	SelectionModeCode string //
	NameZhCn          string //
	NameEnUs          string //
	MinimumSelected   string //
	MaximumSelected   string //
	SortOrder         string //
	CreatedAt         string //
}

// catalogConfigurationGroupColumns holds the columns for the table plugin_idc_core_catalog_configuration_group.
var catalogConfigurationGroupColumns = CatalogConfigurationGroupColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	GroupNo:           "group_no",
	VersionNo:         "version_no",
	GroupCode:         "group_code",
	SelectionModeCode: "selection_mode_code",
	NameZhCn:          "name_zh_cn",
	NameEnUs:          "name_en_us",
	MinimumSelected:   "minimum_selected",
	MaximumSelected:   "maximum_selected",
	SortOrder:         "sort_order",
	CreatedAt:         "created_at",
}

// NewCatalogConfigurationGroupDao creates and returns a new DAO object for table data access.
func NewCatalogConfigurationGroupDao(handlers ...gdb.ModelHandler) *CatalogConfigurationGroupDao {
	return &CatalogConfigurationGroupDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_configuration_group",
		columns:  catalogConfigurationGroupColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogConfigurationGroupDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogConfigurationGroupDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogConfigurationGroupDao) Columns() CatalogConfigurationGroupColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogConfigurationGroupDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogConfigurationGroupDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogConfigurationGroupDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
