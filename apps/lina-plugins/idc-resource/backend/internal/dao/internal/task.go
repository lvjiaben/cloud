// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaskDao is the data access object for the table plugin_idc_resource_task.
type TaskDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TaskColumns        // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TaskColumns defines and stores column names for the table plugin_idc_resource_task.
type TaskColumns struct {
	Id                      string //
	TenantId                string //
	TaskNo                  string //
	RequestIdDigest         string // SHA-256 digest of the caller idempotency key; raw keys are not persisted
	CapabilityCode          string //
	LocalResourceNo         string //
	ChannelNo               string //
	PoolNo                  string //
	StatusCode              string //
	Priority                string //
	SanitizedParameters     string // Validated non-secret operation parameters; credential handles and plaintext are forbidden
	AttemptCount            string //
	MaxAttempts             string //
	AvailableAt             string //
	LockedBy                string //
	LeaseExpiresAt          string //
	NextObserveAt           string //
	AcceptedAt              string //
	FinishedAt              string //
	FailureCode             string //
	FailureSummary          string //
	ResultUnknown           string // Hard replay fence requiring observation or manual reconciliation
	Version                 string //
	CreatedAt               string //
	UpdatedAt               string //
	ObservationClaimedAt    string // PostgreSQL time of the latest QueryResource observation claim; never derived from updated_at
	OwnerMetadata           string // Validated non-secret owner intent retained for replay checks and never sent to the Provider
	ObservationAttemptCount string // Consecutive read-only reconciliation attempts; never authorizes Provider write replay
}

// taskColumns holds the columns for the table plugin_idc_resource_task.
var taskColumns = TaskColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	TaskNo:                  "task_no",
	RequestIdDigest:         "request_id_digest",
	CapabilityCode:          "capability_code",
	LocalResourceNo:         "local_resource_no",
	ChannelNo:               "channel_no",
	PoolNo:                  "pool_no",
	StatusCode:              "status_code",
	Priority:                "priority",
	SanitizedParameters:     "sanitized_parameters",
	AttemptCount:            "attempt_count",
	MaxAttempts:             "max_attempts",
	AvailableAt:             "available_at",
	LockedBy:                "locked_by",
	LeaseExpiresAt:          "lease_expires_at",
	NextObserveAt:           "next_observe_at",
	AcceptedAt:              "accepted_at",
	FinishedAt:              "finished_at",
	FailureCode:             "failure_code",
	FailureSummary:          "failure_summary",
	ResultUnknown:           "result_unknown",
	Version:                 "version",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
	ObservationClaimedAt:    "observation_claimed_at",
	OwnerMetadata:           "owner_metadata",
	ObservationAttemptCount: "observation_attempt_count",
}

// NewTaskDao creates and returns a new DAO object for table data access.
func NewTaskDao(handlers ...gdb.ModelHandler) *TaskDao {
	return &TaskDao{
		group:    "default",
		table:    "plugin_idc_resource_task",
		columns:  taskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaskDao) Columns() TaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
