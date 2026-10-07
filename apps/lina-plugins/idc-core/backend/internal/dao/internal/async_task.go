// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AsyncTaskDao is the data access object for the table plugin_idc_core_async_task.
type AsyncTaskDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  AsyncTaskColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// AsyncTaskColumns defines and stores column names for the table plugin_idc_core_async_task.
type AsyncTaskColumns struct {
	Id                   string //
	TenantId             string //
	TaskNo               string // Opaque task business number
	TaskType             string //
	SubjectType          string //
	SubjectNo            string //
	CustomerNo           string // Validated customer ownership projection for database-side permission filtering
	ExternalReferenceNo  string // Current normalized external task or operation reference
	IdempotencyKeyDigest string // SHA-256 digest of the task idempotency key
	StatusCode           string //
	Priority             string //
	Payload              string // Versioned redacted task input without secrets
	ResultSnapshot       string // Redacted normalized result used for status and recovery
	AvailableAt          string //
	DeadlineAt           string //
	LockedBy             string //
	LeaseExpiresAt       string //
	AttemptCount         string //
	MaxAttempts          string //
	CompletedAt          string //
	ManualTakeoverAt     string // Time when automated execution yielded to an operator
	ManualAssigneeUserId string // Host staff identity assigned to manual takeover
	ManualReason         string // Redacted operator-facing reason for manual takeover
	LastErrorCode        string //
	LastErrorSummary     string //
	CreatedAt            string //
	UpdatedAt            string //
}

// asyncTaskColumns holds the columns for the table plugin_idc_core_async_task.
var asyncTaskColumns = AsyncTaskColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	TaskNo:               "task_no",
	TaskType:             "task_type",
	SubjectType:          "subject_type",
	SubjectNo:            "subject_no",
	CustomerNo:           "customer_no",
	ExternalReferenceNo:  "external_reference_no",
	IdempotencyKeyDigest: "idempotency_key_digest",
	StatusCode:           "status_code",
	Priority:             "priority",
	Payload:              "payload",
	ResultSnapshot:       "result_snapshot",
	AvailableAt:          "available_at",
	DeadlineAt:           "deadline_at",
	LockedBy:             "locked_by",
	LeaseExpiresAt:       "lease_expires_at",
	AttemptCount:         "attempt_count",
	MaxAttempts:          "max_attempts",
	CompletedAt:          "completed_at",
	ManualTakeoverAt:     "manual_takeover_at",
	ManualAssigneeUserId: "manual_assignee_user_id",
	ManualReason:         "manual_reason",
	LastErrorCode:        "last_error_code",
	LastErrorSummary:     "last_error_summary",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewAsyncTaskDao creates and returns a new DAO object for table data access.
func NewAsyncTaskDao(handlers ...gdb.ModelHandler) *AsyncTaskDao {
	return &AsyncTaskDao{
		group:    "default",
		table:    "plugin_idc_core_async_task",
		columns:  asyncTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AsyncTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AsyncTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AsyncTaskDao) Columns() AsyncTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AsyncTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AsyncTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AsyncTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
