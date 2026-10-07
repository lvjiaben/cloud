// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelSecretBindingDao is the data access object for the table plugin_idc_resource_channel_secret_binding.
type ChannelSecretBindingDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ChannelSecretBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ChannelSecretBindingColumns defines and stores column names for the table plugin_idc_resource_channel_secret_binding.
type ChannelSecretBindingColumns struct {
	Id           string //
	TenantId     string //
	BindingNo    string //
	ChannelNo    string //
	BindingName  string // Adapter-defined credential slot such as username or password
	SecretRef    string // Sensitive routing metadata from secretcap Handle.OpaqueRef; never returned, logged, or audited
	StatusCode   string // Recoverable owner lifecycle: preparing, retryable_failed, active, superseded, or revoked
	BoundAt      string //
	SupersededAt string //
	RevokedAt    string //
	Version      string //
	CreatedBy    string //
	UpdatedBy    string //
	CreatedAt    string //
	UpdatedAt    string //
}

// channelSecretBindingColumns holds the columns for the table plugin_idc_resource_channel_secret_binding.
var channelSecretBindingColumns = ChannelSecretBindingColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	BindingNo:    "binding_no",
	ChannelNo:    "channel_no",
	BindingName:  "binding_name",
	SecretRef:    "secret_ref",
	StatusCode:   "status_code",
	BoundAt:      "bound_at",
	SupersededAt: "superseded_at",
	RevokedAt:    "revoked_at",
	Version:      "version",
	CreatedBy:    "created_by",
	UpdatedBy:    "updated_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewChannelSecretBindingDao creates and returns a new DAO object for table data access.
func NewChannelSecretBindingDao(handlers ...gdb.ModelHandler) *ChannelSecretBindingDao {
	return &ChannelSecretBindingDao{
		group:    "default",
		table:    "plugin_idc_resource_channel_secret_binding",
		columns:  channelSecretBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelSecretBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelSecretBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelSecretBindingDao) Columns() ChannelSecretBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelSecretBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelSecretBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelSecretBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
