// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerOperationExecutionDao is the data access object for the table plugin_idc_core_server_operation_execution.
type ServerOperationExecutionDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  ServerOperationExecutionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// ServerOperationExecutionColumns defines and stores column names for the table plugin_idc_core_server_operation_execution.
type ServerOperationExecutionColumns struct {
	Id                        string //
	TenantId                  string //
	ExecutionNo               string //
	OperationNo               string //
	ServiceNo                 string //
	ActionCode                string //
	ResourceOwnerCode         string //
	ChannelNo                 string //
	PoolNo                    string //
	ExpectedServiceVersion    string //
	OriginalRuntimeStatusCode string //
	Parameters                string //
	SecretBindings            string // Opaque input secret references only; plaintext is forbidden
	StatusCode                string //
	ProviderTaskNo            string //
	RequestDigest             string //
	CapabilityEvidenceNo      string //
	CapabilityEvidenceDigest  string //
	TaskEvidenceDigest        string //
	LastFailureCode           string //
	OutputBindings            string // Opaque confidential result references with one-time or short-lived delivery policy
	ObservedAt                string //
	CompletedAt               string //
	Version                   string //
	CreatedAt                 string //
	UpdatedAt                 string //
}

// serverOperationExecutionColumns holds the columns for the table plugin_idc_core_server_operation_execution.
var serverOperationExecutionColumns = ServerOperationExecutionColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	ExecutionNo:               "execution_no",
	OperationNo:               "operation_no",
	ServiceNo:                 "service_no",
	ActionCode:                "action_code",
	ResourceOwnerCode:         "resource_owner_code",
	ChannelNo:                 "channel_no",
	PoolNo:                    "pool_no",
	ExpectedServiceVersion:    "expected_service_version",
	OriginalRuntimeStatusCode: "original_runtime_status_code",
	Parameters:                "parameters",
	SecretBindings:            "secret_bindings",
	StatusCode:                "status_code",
	ProviderTaskNo:            "provider_task_no",
	RequestDigest:             "request_digest",
	CapabilityEvidenceNo:      "capability_evidence_no",
	CapabilityEvidenceDigest:  "capability_evidence_digest",
	TaskEvidenceDigest:        "task_evidence_digest",
	LastFailureCode:           "last_failure_code",
	OutputBindings:            "output_bindings",
	ObservedAt:                "observed_at",
	CompletedAt:               "completed_at",
	Version:                   "version",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
}

// NewServerOperationExecutionDao creates and returns a new DAO object for table data access.
func NewServerOperationExecutionDao(handlers ...gdb.ModelHandler) *ServerOperationExecutionDao {
	return &ServerOperationExecutionDao{
		group:    "default",
		table:    "plugin_idc_core_server_operation_execution",
		columns:  serverOperationExecutionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerOperationExecutionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerOperationExecutionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerOperationExecutionDao) Columns() ServerOperationExecutionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerOperationExecutionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerOperationExecutionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerOperationExecutionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
