// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ManualResendDao is the data access object for the table plugin_idc_notify_manual_resend.
type ManualResendDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  ManualResendColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// ManualResendColumns defines and stores column names for the table plugin_idc_notify_manual_resend.
type ManualResendColumns struct {
	TenantId          string //
	OperationNo       string //
	ChainNo           string //
	SourceTaskNo      string //
	CreatedTaskNo     string //
	ActorUserId       string //
	ReasonText        string //
	SourceTaskVersion string //
	CreatedAt         string //
}

// manualResendColumns holds the columns for the table plugin_idc_notify_manual_resend.
var manualResendColumns = ManualResendColumns{
	TenantId:          "tenant_id",
	OperationNo:       "operation_no",
	ChainNo:           "chain_no",
	SourceTaskNo:      "source_task_no",
	CreatedTaskNo:     "created_task_no",
	ActorUserId:       "actor_user_id",
	ReasonText:        "reason_text",
	SourceTaskVersion: "source_task_version",
	CreatedAt:         "created_at",
}

// NewManualResendDao creates and returns a new DAO object for table data access.
func NewManualResendDao(handlers ...gdb.ModelHandler) *ManualResendDao {
	return &ManualResendDao{
		group:    "default",
		table:    "plugin_idc_notify_manual_resend",
		columns:  manualResendColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ManualResendDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ManualResendDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ManualResendDao) Columns() ManualResendColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ManualResendDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ManualResendDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ManualResendDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
