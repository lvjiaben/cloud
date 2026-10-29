// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// LoginContactGuardDao is the data access object for the table plugin_idc_notify_login_contact_guard.
type LoginContactGuardDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  LoginContactGuardColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// LoginContactGuardColumns defines and stores column names for the table plugin_idc_notify_login_contact_guard.
type LoginContactGuardColumns struct {
	TenantId           string //
	UserId             string //
	Revision           string //
	CreatedAt          string //
	UpdatedAt          string //
	RecoveryNo         string //
	CleanupPending     string //
	RecoveryAt         string //
	RecoveryRevision   string //
	CleanupTokenDigest string //
}

// loginContactGuardColumns holds the columns for the table plugin_idc_notify_login_contact_guard.
var loginContactGuardColumns = LoginContactGuardColumns{
	TenantId:           "tenant_id",
	UserId:             "user_id",
	Revision:           "revision",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
	RecoveryNo:         "recovery_no",
	CleanupPending:     "cleanup_pending",
	RecoveryAt:         "recovery_at",
	RecoveryRevision:   "recovery_revision",
	CleanupTokenDigest: "cleanup_token_digest",
}

// NewLoginContactGuardDao creates and returns a new DAO object for table data access.
func NewLoginContactGuardDao(handlers ...gdb.ModelHandler) *LoginContactGuardDao {
	return &LoginContactGuardDao{
		group:    "default",
		table:    "plugin_idc_notify_login_contact_guard",
		columns:  loginContactGuardColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *LoginContactGuardDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *LoginContactGuardDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *LoginContactGuardDao) Columns() LoginContactGuardColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *LoginContactGuardDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *LoginContactGuardDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *LoginContactGuardDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
