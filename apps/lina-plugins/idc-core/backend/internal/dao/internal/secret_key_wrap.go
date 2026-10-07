// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SecretKeyWrapDao is the data access object for the table plugin_idc_core_secret_key_wrap.
type SecretKeyWrapDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SecretKeyWrapColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SecretKeyWrapColumns defines and stores column names for the table plugin_idc_core_secret_key_wrap.
type SecretKeyWrapColumns struct {
	Id              string //
	TenantId        string //
	SecretNo        string //
	SecretVersion   string //
	EnvelopeVersion string //
	MasterKeyId     string // External deployment key identifier; master-key bytes are never persisted
	WrapAlgorithm   string //
	WrapNonce       string // Canonical hexadecimal encoding of the random 96-bit AES-GCM wrapper nonce
	WrappedDek      string // Canonical hexadecimal authenticated 32-byte DEK ciphertext plus the 16-byte GCM tag
	CreatedAt       string //
}

// secretKeyWrapColumns holds the columns for the table plugin_idc_core_secret_key_wrap.
var secretKeyWrapColumns = SecretKeyWrapColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	SecretNo:        "secret_no",
	SecretVersion:   "secret_version",
	EnvelopeVersion: "envelope_version",
	MasterKeyId:     "master_key_id",
	WrapAlgorithm:   "wrap_algorithm",
	WrapNonce:       "wrap_nonce",
	WrappedDek:      "wrapped_dek",
	CreatedAt:       "created_at",
}

// NewSecretKeyWrapDao creates and returns a new DAO object for table data access.
func NewSecretKeyWrapDao(handlers ...gdb.ModelHandler) *SecretKeyWrapDao {
	return &SecretKeyWrapDao{
		group:    "default",
		table:    "plugin_idc_core_secret_key_wrap",
		columns:  secretKeyWrapColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SecretKeyWrapDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SecretKeyWrapDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SecretKeyWrapDao) Columns() SecretKeyWrapColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SecretKeyWrapDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SecretKeyWrapDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SecretKeyWrapDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
