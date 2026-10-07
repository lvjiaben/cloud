// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SecretRotationStateDao is the data access object for the table plugin_idc_core_secret_rotation_state.
type SecretRotationStateDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SecretRotationStateColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SecretRotationStateColumns defines and stores column names for the table plugin_idc_core_secret_rotation_state.
type SecretRotationStateColumns struct {
	Id                     string //
	TenantId               string //
	OwnerPluginId          string //
	CallerScope            string //
	Epoch                  string // Monotonic active-key verification generation
	ActiveKeyId            string // Active key identifier certified by the current generation
	StatusCode             string //
	WriteFence             string // Monotonic payload mutation fence shared by all service instances
	VerificationCursorId   string // Last immutable payload row authenticated in the current generation
	VerificationWriteFence string // Write fence captured when the current full verification pass began
	CreatedAt              string //
	UpdatedAt              string //
}

// secretRotationStateColumns holds the columns for the table plugin_idc_core_secret_rotation_state.
var secretRotationStateColumns = SecretRotationStateColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	OwnerPluginId:          "owner_plugin_id",
	CallerScope:            "caller_scope",
	Epoch:                  "epoch",
	ActiveKeyId:            "active_key_id",
	StatusCode:             "status_code",
	WriteFence:             "write_fence",
	VerificationCursorId:   "verification_cursor_id",
	VerificationWriteFence: "verification_write_fence",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
}

// NewSecretRotationStateDao creates and returns a new DAO object for table data access.
func NewSecretRotationStateDao(handlers ...gdb.ModelHandler) *SecretRotationStateDao {
	return &SecretRotationStateDao{
		group:    "default",
		table:    "plugin_idc_core_secret_rotation_state",
		columns:  secretRotationStateColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SecretRotationStateDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SecretRotationStateDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SecretRotationStateDao) Columns() SecretRotationStateColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SecretRotationStateDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SecretRotationStateDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SecretRotationStateDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
