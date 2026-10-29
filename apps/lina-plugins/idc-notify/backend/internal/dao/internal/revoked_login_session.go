// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RevokedLoginSessionDao is the data access object for the table plugin_idc_notify_revoked_login_session.
type RevokedLoginSessionDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  RevokedLoginSessionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// RevokedLoginSessionColumns defines and stores column names for the table plugin_idc_notify_revoked_login_session.
type RevokedLoginSessionColumns struct {
	TenantId      string //
	UserId        string //
	SessionDigest string //
	SessionId     string //
	RecoveryNo    string //
	CleanupDone   string //
	CreatedAt     string //
	UpdatedAt     string //
}

// revokedLoginSessionColumns holds the columns for the table plugin_idc_notify_revoked_login_session.
var revokedLoginSessionColumns = RevokedLoginSessionColumns{
	TenantId:      "tenant_id",
	UserId:        "user_id",
	SessionDigest: "session_digest",
	SessionId:     "session_id",
	RecoveryNo:    "recovery_no",
	CleanupDone:   "cleanup_done",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewRevokedLoginSessionDao creates and returns a new DAO object for table data access.
func NewRevokedLoginSessionDao(handlers ...gdb.ModelHandler) *RevokedLoginSessionDao {
	return &RevokedLoginSessionDao{
		group:    "default",
		table:    "plugin_idc_notify_revoked_login_session",
		columns:  revokedLoginSessionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RevokedLoginSessionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RevokedLoginSessionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RevokedLoginSessionDao) Columns() RevokedLoginSessionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RevokedLoginSessionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RevokedLoginSessionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RevokedLoginSessionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
