// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaskEvidenceDao is the data access object for the table plugin_idc_resource_task_evidence.
type TaskEvidenceDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  TaskEvidenceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// TaskEvidenceColumns defines and stores column names for the table plugin_idc_resource_task_evidence.
type TaskEvidenceColumns struct {
	Id           string //
	TenantId     string //
	EvidenceNo   string //
	TaskNo       string //
	SourceKind   string //
	EvidenceKind string //
	Reference    string // Bounded upstream task or resource reference; never a secret handle or raw payload
	Summary      string // Bounded credential-scanned Provider or owner summary; never a raw response
	ObservedAt   string //
	CreatedAt    string //
}

// taskEvidenceColumns holds the columns for the table plugin_idc_resource_task_evidence.
var taskEvidenceColumns = TaskEvidenceColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	EvidenceNo:   "evidence_no",
	TaskNo:       "task_no",
	SourceKind:   "source_kind",
	EvidenceKind: "evidence_kind",
	Reference:    "reference",
	Summary:      "summary",
	ObservedAt:   "observed_at",
	CreatedAt:    "created_at",
}

// NewTaskEvidenceDao creates and returns a new DAO object for table data access.
func NewTaskEvidenceDao(handlers ...gdb.ModelHandler) *TaskEvidenceDao {
	return &TaskEvidenceDao{
		group:    "default",
		table:    "plugin_idc_resource_task_evidence",
		columns:  taskEvidenceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaskEvidenceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaskEvidenceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaskEvidenceDao) Columns() TaskEvidenceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaskEvidenceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaskEvidenceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaskEvidenceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
