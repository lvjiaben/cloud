// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ManualClosureDao is the data access object for the table plugin_idc_notify_manual_closure.
type ManualClosureDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  ManualClosureColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// ManualClosureColumns defines and stores column names for the table plugin_idc_notify_manual_closure.
type ManualClosureColumns struct {
	TenantId            string //
	ClosureNo           string //
	TaskNo              string //
	ActorUserId         string //
	ReasonText          string //
	ExpectedTaskVersion string //
	CreatedAt           string //
}

// manualClosureColumns holds the columns for the table plugin_idc_notify_manual_closure.
var manualClosureColumns = ManualClosureColumns{
	TenantId:            "tenant_id",
	ClosureNo:           "closure_no",
	TaskNo:              "task_no",
	ActorUserId:         "actor_user_id",
	ReasonText:          "reason_text",
	ExpectedTaskVersion: "expected_task_version",
	CreatedAt:           "created_at",
}

// NewManualClosureDao creates and returns a new DAO object for table data access.
func NewManualClosureDao(handlers ...gdb.ModelHandler) *ManualClosureDao {
	return &ManualClosureDao{
		group:    "default",
		table:    "plugin_idc_notify_manual_closure",
		columns:  manualClosureColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ManualClosureDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ManualClosureDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ManualClosureDao) Columns() ManualClosureColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ManualClosureDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ManualClosureDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ManualClosureDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
