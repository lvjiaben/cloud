// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CapabilitySnapshotItemDao is the data access object for the table plugin_idc_resource_capability_snapshot_item.
type CapabilitySnapshotItemDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  CapabilitySnapshotItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// CapabilitySnapshotItemColumns defines and stores column names for the table plugin_idc_resource_capability_snapshot_item.
type CapabilitySnapshotItemColumns struct {
	Id                string //
	TenantId          string //
	SnapshotNo        string //
	CapabilityCode    string //
	SupportCode       string //
	Constraints       string //
	UnsupportedReason string //
	CreatedAt         string //
}

// capabilitySnapshotItemColumns holds the columns for the table plugin_idc_resource_capability_snapshot_item.
var capabilitySnapshotItemColumns = CapabilitySnapshotItemColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	SnapshotNo:        "snapshot_no",
	CapabilityCode:    "capability_code",
	SupportCode:       "support_code",
	Constraints:       "constraints",
	UnsupportedReason: "unsupported_reason",
	CreatedAt:         "created_at",
}

// NewCapabilitySnapshotItemDao creates and returns a new DAO object for table data access.
func NewCapabilitySnapshotItemDao(handlers ...gdb.ModelHandler) *CapabilitySnapshotItemDao {
	return &CapabilitySnapshotItemDao{
		group:    "default",
		table:    "plugin_idc_resource_capability_snapshot_item",
		columns:  capabilitySnapshotItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CapabilitySnapshotItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CapabilitySnapshotItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CapabilitySnapshotItemDao) Columns() CapabilitySnapshotItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CapabilitySnapshotItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CapabilitySnapshotItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CapabilitySnapshotItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
