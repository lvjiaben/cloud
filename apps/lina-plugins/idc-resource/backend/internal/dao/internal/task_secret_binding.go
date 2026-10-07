// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaskSecretBindingDao is the data access object for the table plugin_idc_resource_task_secret_binding.
type TaskSecretBindingDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  TaskSecretBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// TaskSecretBindingColumns defines and stores column names for the table plugin_idc_resource_task_secret_binding.
type TaskSecretBindingColumns struct {
	Id               string //
	TenantId         string //
	BindingNo        string //
	TaskNo           string //
	BindingName      string //
	BindingDirection string //
	SecretRef        string // Sensitive secretcap routing reference excluded from task JSON, logs, audit, and API responses
	OutputPolicy     string // One-time credential or short-lived ticket delivery policy for output bindings
	ExpiresAt        string //
	StatusCode       string //
	RevealedAt       string //
	RevokedAt        string //
	Version          string //
	CreatedAt        string //
	UpdatedAt        string //
}

// taskSecretBindingColumns holds the columns for the table plugin_idc_resource_task_secret_binding.
var taskSecretBindingColumns = TaskSecretBindingColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	BindingNo:        "binding_no",
	TaskNo:           "task_no",
	BindingName:      "binding_name",
	BindingDirection: "binding_direction",
	SecretRef:        "secret_ref",
	OutputPolicy:     "output_policy",
	ExpiresAt:        "expires_at",
	StatusCode:       "status_code",
	RevealedAt:       "revealed_at",
	RevokedAt:        "revoked_at",
	Version:          "version",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewTaskSecretBindingDao creates and returns a new DAO object for table data access.
func NewTaskSecretBindingDao(handlers ...gdb.ModelHandler) *TaskSecretBindingDao {
	return &TaskSecretBindingDao{
		group:    "default",
		table:    "plugin_idc_resource_task_secret_binding",
		columns:  taskSecretBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaskSecretBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaskSecretBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaskSecretBindingDao) Columns() TaskSecretBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaskSecretBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaskSecretBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaskSecretBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
