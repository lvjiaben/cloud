// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AsyncTaskAttemptDao is the data access object for the table plugin_idc_core_async_task_attempt.
type AsyncTaskAttemptDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  AsyncTaskAttemptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// AsyncTaskAttemptColumns defines and stores column names for the table plugin_idc_core_async_task_attempt.
type AsyncTaskAttemptColumns struct {
	Id                  string //
	TenantId            string //
	TaskNo              string //
	AttemptNo           string // Monotonic attempt number within one task
	WorkerId            string //
	ExternalReferenceNo string // External reference returned for this execution attempt
	StatusCode          string //
	RequestSnapshot     string // Redacted normalized request evidence
	ResponseSnapshot    string // Redacted normalized response evidence
	StartedAt           string //
	EndedAt             string //
	ErrorCode           string //
	ErrorSummary        string //
	CreatedAt           string //
	UpdatedAt           string //
}

// asyncTaskAttemptColumns holds the columns for the table plugin_idc_core_async_task_attempt.
var asyncTaskAttemptColumns = AsyncTaskAttemptColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	TaskNo:              "task_no",
	AttemptNo:           "attempt_no",
	WorkerId:            "worker_id",
	ExternalReferenceNo: "external_reference_no",
	StatusCode:          "status_code",
	RequestSnapshot:     "request_snapshot",
	ResponseSnapshot:    "response_snapshot",
	StartedAt:           "started_at",
	EndedAt:             "ended_at",
	ErrorCode:           "error_code",
	ErrorSummary:        "error_summary",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewAsyncTaskAttemptDao creates and returns a new DAO object for table data access.
func NewAsyncTaskAttemptDao(handlers ...gdb.ModelHandler) *AsyncTaskAttemptDao {
	return &AsyncTaskAttemptDao{
		group:    "default",
		table:    "plugin_idc_core_async_task_attempt",
		columns:  asyncTaskAttemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AsyncTaskAttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AsyncTaskAttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AsyncTaskAttemptDao) Columns() AsyncTaskAttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AsyncTaskAttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AsyncTaskAttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AsyncTaskAttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
