// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceConfigurationSnapshotItemDao is the data access object for the table plugin_idc_core_commerce_configuration_snapshot_item.
type CommerceConfigurationSnapshotItemDao struct {
	table    string                                   // table is the underlying table name of the DAO.
	group    string                                   // group is the database configuration group name of the current DAO.
	columns  CommerceConfigurationSnapshotItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                       // handlers for customized model modification.
}

// CommerceConfigurationSnapshotItemColumns defines and stores column names for the table plugin_idc_core_commerce_configuration_snapshot_item.
type CommerceConfigurationSnapshotItemColumns struct {
	Id                      string //
	TenantId                string //
	SnapshotItemNo          string //
	ConfigurationSnapshotNo string //
	VersionNo               string //
	GroupNo                 string //
	OptionNo                string //
	GroupCode               string //
	OptionCode              string //
	ComponentTypeCode       string //
	GroupNameZhCn           string //
	GroupNameEnUs           string //
	OptionNameZhCn          string //
	OptionNameEnUs          string //
	NormalizedValue         string //
	Quantity                string //
	SortOrder               string //
	CreatedAt               string //
}

// commerceConfigurationSnapshotItemColumns holds the columns for the table plugin_idc_core_commerce_configuration_snapshot_item.
var commerceConfigurationSnapshotItemColumns = CommerceConfigurationSnapshotItemColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	SnapshotItemNo:          "snapshot_item_no",
	ConfigurationSnapshotNo: "configuration_snapshot_no",
	VersionNo:               "version_no",
	GroupNo:                 "group_no",
	OptionNo:                "option_no",
	GroupCode:               "group_code",
	OptionCode:              "option_code",
	ComponentTypeCode:       "component_type_code",
	GroupNameZhCn:           "group_name_zh_cn",
	GroupNameEnUs:           "group_name_en_us",
	OptionNameZhCn:          "option_name_zh_cn",
	OptionNameEnUs:          "option_name_en_us",
	NormalizedValue:         "normalized_value",
	Quantity:                "quantity",
	SortOrder:               "sort_order",
	CreatedAt:               "created_at",
}

// NewCommerceConfigurationSnapshotItemDao creates and returns a new DAO object for table data access.
func NewCommerceConfigurationSnapshotItemDao(handlers ...gdb.ModelHandler) *CommerceConfigurationSnapshotItemDao {
	return &CommerceConfigurationSnapshotItemDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_configuration_snapshot_item",
		columns:  commerceConfigurationSnapshotItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceConfigurationSnapshotItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceConfigurationSnapshotItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceConfigurationSnapshotItemDao) Columns() CommerceConfigurationSnapshotItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceConfigurationSnapshotItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceConfigurationSnapshotItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceConfigurationSnapshotItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
