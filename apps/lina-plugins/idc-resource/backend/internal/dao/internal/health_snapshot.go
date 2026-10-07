// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// HealthSnapshotDao is the data access object for the table plugin_idc_resource_health_snapshot.
type HealthSnapshotDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  HealthSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// HealthSnapshotColumns defines and stores column names for the table plugin_idc_resource_health_snapshot.
type HealthSnapshotColumns struct {
	Id                         string //
	TenantId                   string //
	SnapshotNo                 string //
	ChannelNo                  string //
	HealthCode                 string //
	LatencyMs                  string //
	ConsecutiveFailures        string //
	CircuitOpen                string //
	CapabilityObservedAt       string //
	ProviderCapacityObservedAt string //
	SummaryCode                string // Stable non-sensitive machine code; raw upstream errors are forbidden
	ObservedAt                 string //
	CreatedAt                  string //
	OperationKind              string //
}

// healthSnapshotColumns holds the columns for the table plugin_idc_resource_health_snapshot.
var healthSnapshotColumns = HealthSnapshotColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	SnapshotNo:                 "snapshot_no",
	ChannelNo:                  "channel_no",
	HealthCode:                 "health_code",
	LatencyMs:                  "latency_ms",
	ConsecutiveFailures:        "consecutive_failures",
	CircuitOpen:                "circuit_open",
	CapabilityObservedAt:       "capability_observed_at",
	ProviderCapacityObservedAt: "provider_capacity_observed_at",
	SummaryCode:                "summary_code",
	ObservedAt:                 "observed_at",
	CreatedAt:                  "created_at",
	OperationKind:              "operation_kind",
}

// NewHealthSnapshotDao creates and returns a new DAO object for table data access.
func NewHealthSnapshotDao(handlers ...gdb.ModelHandler) *HealthSnapshotDao {
	return &HealthSnapshotDao{
		group:    "default",
		table:    "plugin_idc_resource_health_snapshot",
		columns:  healthSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *HealthSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *HealthSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *HealthSnapshotDao) Columns() HealthSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *HealthSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *HealthSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *HealthSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
