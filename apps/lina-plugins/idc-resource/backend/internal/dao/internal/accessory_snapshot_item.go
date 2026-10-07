// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AccessorySnapshotItemDao is the data access object for the table plugin_idc_resource_accessory_snapshot_item.
type AccessorySnapshotItemDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  AccessorySnapshotItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// AccessorySnapshotItemColumns defines and stores column names for the table plugin_idc_resource_accessory_snapshot_item.
type AccessorySnapshotItemColumns struct {
	Id            string //
	TenantId      string //
	SnapshotNo    string //
	ObjectNo      string // Local opaque object number exposed to callers instead of Provider identifiers
	AccessoryKind string //
	ExternalRef   string // Resource-owner private Provider reference; never returned, logged, or audited
	DisplayName   string //
	StateCode     string //
	Projection    string //
	ObservedAt    string //
	CreatedAt     string //
}

// accessorySnapshotItemColumns holds the columns for the table plugin_idc_resource_accessory_snapshot_item.
var accessorySnapshotItemColumns = AccessorySnapshotItemColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	SnapshotNo:    "snapshot_no",
	ObjectNo:      "object_no",
	AccessoryKind: "accessory_kind",
	ExternalRef:   "external_ref",
	DisplayName:   "display_name",
	StateCode:     "state_code",
	Projection:    "projection",
	ObservedAt:    "observed_at",
	CreatedAt:     "created_at",
}

// NewAccessorySnapshotItemDao creates and returns a new DAO object for table data access.
func NewAccessorySnapshotItemDao(handlers ...gdb.ModelHandler) *AccessorySnapshotItemDao {
	return &AccessorySnapshotItemDao{
		group:    "default",
		table:    "plugin_idc_resource_accessory_snapshot_item",
		columns:  accessorySnapshotItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AccessorySnapshotItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AccessorySnapshotItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AccessorySnapshotItemDao) Columns() AccessorySnapshotItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AccessorySnapshotItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AccessorySnapshotItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AccessorySnapshotItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
