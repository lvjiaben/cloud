// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AccessorySnapshotDao is the data access object for the table plugin_idc_resource_accessory_snapshot.
type AccessorySnapshotDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  AccessorySnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// AccessorySnapshotColumns defines and stores column names for the table plugin_idc_resource_accessory_snapshot.
type AccessorySnapshotColumns struct {
	Id                  string //
	TenantId            string //
	SnapshotNo          string //
	MappingNo           string //
	LocalResourceNo     string //
	ChannelNo           string //
	StatusCode          string //
	SnapshotFingerprint string //
	ItemCount           string //
	ObservedAt          string //
	ExpiresAt           string //
	CreatedAt           string //
}

// accessorySnapshotColumns holds the columns for the table plugin_idc_resource_accessory_snapshot.
var accessorySnapshotColumns = AccessorySnapshotColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	SnapshotNo:          "snapshot_no",
	MappingNo:           "mapping_no",
	LocalResourceNo:     "local_resource_no",
	ChannelNo:           "channel_no",
	StatusCode:          "status_code",
	SnapshotFingerprint: "snapshot_fingerprint",
	ItemCount:           "item_count",
	ObservedAt:          "observed_at",
	ExpiresAt:           "expires_at",
	CreatedAt:           "created_at",
}

// NewAccessorySnapshotDao creates and returns a new DAO object for table data access.
func NewAccessorySnapshotDao(handlers ...gdb.ModelHandler) *AccessorySnapshotDao {
	return &AccessorySnapshotDao{
		group:    "default",
		table:    "plugin_idc_resource_accessory_snapshot",
		columns:  accessorySnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AccessorySnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AccessorySnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AccessorySnapshotDao) Columns() AccessorySnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AccessorySnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AccessorySnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AccessorySnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
