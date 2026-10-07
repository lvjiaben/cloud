// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelDao is the data access object for the table plugin_idc_notify_channel.
type ChannelDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ChannelColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ChannelColumns defines and stores column names for the table plugin_idc_notify_channel.
type ChannelColumns struct {
	TenantId                   string //
	ChannelCode                string //
	ChannelKind                string //
	ProviderCode               string //
	DisplayName                string //
	StatusCode                 string //
	BalanceState               string //
	ConfigurationGeneration    string //
	ConfigurationVersion       string //
	NonSecretConfiguration     string //
	CredentialSecretRef        string //
	CredentialMaskedTail       string //
	CredentialUpdatedAt        string //
	CallbackVerifiedGeneration string //
	CallbackVerifiedAt         string //
	DisabledAt                 string //
	UpdatedAt                  string //
	CreatedAt                  string //
}

// channelColumns holds the columns for the table plugin_idc_notify_channel.
var channelColumns = ChannelColumns{
	TenantId:                   "tenant_id",
	ChannelCode:                "channel_code",
	ChannelKind:                "channel_kind",
	ProviderCode:               "provider_code",
	DisplayName:                "display_name",
	StatusCode:                 "status_code",
	BalanceState:               "balance_state",
	ConfigurationGeneration:    "configuration_generation",
	ConfigurationVersion:       "configuration_version",
	NonSecretConfiguration:     "non_secret_configuration",
	CredentialSecretRef:        "credential_secret_ref",
	CredentialMaskedTail:       "credential_masked_tail",
	CredentialUpdatedAt:        "credential_updated_at",
	CallbackVerifiedGeneration: "callback_verified_generation",
	CallbackVerifiedAt:         "callback_verified_at",
	DisabledAt:                 "disabled_at",
	UpdatedAt:                  "updated_at",
	CreatedAt:                  "created_at",
}

// NewChannelDao creates and returns a new DAO object for table data access.
func NewChannelDao(handlers ...gdb.ModelHandler) *ChannelDao {
	return &ChannelDao{
		group:    "default",
		table:    "plugin_idc_notify_channel",
		columns:  channelColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelDao) Columns() ChannelColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
