// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CapabilitySnapshotDao is the data access object for the table plugin_idc_resource_capability_snapshot.
type CapabilitySnapshotDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  CapabilitySnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// CapabilitySnapshotColumns defines and stores column names for the table plugin_idc_resource_capability_snapshot.
type CapabilitySnapshotColumns struct {
	Id             string //
	TenantId       string //
	SnapshotNo     string //
	ChannelNo      string //
	AdapterVersion string //
	StatusCode     string //
	ItemCount      string //
	ObservedAt     string //
	CompletedAt    string //
	CreatedAt      string //
}

// capabilitySnapshotColumns holds the columns for the table plugin_idc_resource_capability_snapshot.
var capabilitySnapshotColumns = CapabilitySnapshotColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	SnapshotNo:     "snapshot_no",
	ChannelNo:      "channel_no",
	AdapterVersion: "adapter_version",
	StatusCode:     "status_code",
	ItemCount:      "item_count",
	ObservedAt:     "observed_at",
	CompletedAt:    "completed_at",
	CreatedAt:      "created_at",
}

// NewCapabilitySnapshotDao creates and returns a new DAO object for table data access.
func NewCapabilitySnapshotDao(handlers ...gdb.ModelHandler) *CapabilitySnapshotDao {
	return &CapabilitySnapshotDao{
		group:    "default",
		table:    "plugin_idc_resource_capability_snapshot",
		columns:  capabilitySnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CapabilitySnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CapabilitySnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CapabilitySnapshotDao) Columns() CapabilitySnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CapabilitySnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CapabilitySnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CapabilitySnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
