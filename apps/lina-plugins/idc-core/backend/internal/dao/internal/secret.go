// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SecretDao is the data access object for the table plugin_idc_core_secret.
type SecretDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SecretColumns      // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SecretColumns defines and stores column names for the table plugin_idc_core_secret.
type SecretColumns struct {
	Id             string //
	TenantId       string //
	SecretNo       string // Opaque secret business number reserved by the core business-number ledger
	OwnerPluginId  string // Plugin that owns and authorizes the secret binding
	CallerScope    string // Exact least-privilege caller scope bound into encryption AAD
	StatusCode     string // Secret lifecycle state: configured or revoked
	CurrentVersion string // Current immutable encrypted payload version
	MaskedTail     string // Non-secret fixed-prefix display mask with at most four printable trailing bytes
	RevokedAt      string // Time after which callback access is denied
	CreatedAt      string //
	UpdatedAt      string //
}

// secretColumns holds the columns for the table plugin_idc_core_secret.
var secretColumns = SecretColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	SecretNo:       "secret_no",
	OwnerPluginId:  "owner_plugin_id",
	CallerScope:    "caller_scope",
	StatusCode:     "status_code",
	CurrentVersion: "current_version",
	MaskedTail:     "masked_tail",
	RevokedAt:      "revoked_at",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewSecretDao creates and returns a new DAO object for table data access.
func NewSecretDao(handlers ...gdb.ModelHandler) *SecretDao {
	return &SecretDao{
		group:    "default",
		table:    "plugin_idc_core_secret",
		columns:  secretColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SecretDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SecretDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SecretDao) Columns() SecretColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SecretDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SecretDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SecretDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
