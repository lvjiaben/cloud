// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// InterfaceCandidateSnapshotDao is the data access object for the table plugin_idc_resource_interface_candidate_snapshot.
type InterfaceCandidateSnapshotDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  InterfaceCandidateSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// InterfaceCandidateSnapshotColumns defines and stores column names for the table plugin_idc_resource_interface_candidate_snapshot.
type InterfaceCandidateSnapshotColumns struct {
	RequestKey            string //
	TenantId              string //
	InterfaceInstanceNo   string //
	ConfigurationRevision string //
	RequestPayload        string //
	ResultPayload         string //
	ObservedAt            string //
	RefreshAfter          string //
	LeaseUntil            string //
	LastErrorCode         string //
}

// interfaceCandidateSnapshotColumns holds the columns for the table plugin_idc_resource_interface_candidate_snapshot.
var interfaceCandidateSnapshotColumns = InterfaceCandidateSnapshotColumns{
	RequestKey:            "request_key",
	TenantId:              "tenant_id",
	InterfaceInstanceNo:   "interface_instance_no",
	ConfigurationRevision: "configuration_revision",
	RequestPayload:        "request_payload",
	ResultPayload:         "result_payload",
	ObservedAt:            "observed_at",
	RefreshAfter:          "refresh_after",
	LeaseUntil:            "lease_until",
	LastErrorCode:         "last_error_code",
}

// NewInterfaceCandidateSnapshotDao creates and returns a new DAO object for table data access.
func NewInterfaceCandidateSnapshotDao(handlers ...gdb.ModelHandler) *InterfaceCandidateSnapshotDao {
	return &InterfaceCandidateSnapshotDao{
		group:    "default",
		table:    "plugin_idc_resource_interface_candidate_snapshot",
		columns:  interfaceCandidateSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *InterfaceCandidateSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *InterfaceCandidateSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *InterfaceCandidateSnapshotDao) Columns() InterfaceCandidateSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *InterfaceCandidateSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *InterfaceCandidateSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *InterfaceCandidateSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
