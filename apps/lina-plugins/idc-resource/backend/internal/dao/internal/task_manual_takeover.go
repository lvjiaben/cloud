// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaskManualTakeoverDao is the data access object for the table plugin_idc_resource_task_manual_takeover.
type TaskManualTakeoverDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  TaskManualTakeoverColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// TaskManualTakeoverColumns defines and stores column names for the table plugin_idc_resource_task_manual_takeover.
type TaskManualTakeoverColumns struct {
	Id             string //
	TenantId       string //
	TakeoverNo     string //
	TaskNo         string //
	FromStatusCode string //
	TaskVersion    string //
	ReasonCode     string //
	RequestDigest  string // Canonical digest of actor, task version, reason, and sorted evidence numbers for exact safe replay
	EvidenceDigest string // Canonical digest of immutable task evidence numbers and their redacted contents
	EvidenceCount  string //
	AssignedUserId string //
	CreatedAt      string //
}

// taskManualTakeoverColumns holds the columns for the table plugin_idc_resource_task_manual_takeover.
var taskManualTakeoverColumns = TaskManualTakeoverColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	TakeoverNo:     "takeover_no",
	TaskNo:         "task_no",
	FromStatusCode: "from_status_code",
	TaskVersion:    "task_version",
	ReasonCode:     "reason_code",
	RequestDigest:  "request_digest",
	EvidenceDigest: "evidence_digest",
	EvidenceCount:  "evidence_count",
	AssignedUserId: "assigned_user_id",
	CreatedAt:      "created_at",
}

// NewTaskManualTakeoverDao creates and returns a new DAO object for table data access.
func NewTaskManualTakeoverDao(handlers ...gdb.ModelHandler) *TaskManualTakeoverDao {
	return &TaskManualTakeoverDao{
		group:    "default",
		table:    "plugin_idc_resource_task_manual_takeover",
		columns:  taskManualTakeoverColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaskManualTakeoverDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaskManualTakeoverDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaskManualTakeoverDao) Columns() TaskManualTakeoverColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaskManualTakeoverDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaskManualTakeoverDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaskManualTakeoverDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
