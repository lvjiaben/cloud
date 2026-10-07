// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelDao is the data access object for the table plugin_idc_resource_channel.
type ChannelDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ChannelColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ChannelColumns defines and stores column names for the table plugin_idc_resource_channel.
type ChannelColumns struct {
	Id                        string //
	TenantId                  string //
	ChannelNo                 string // Opaque resource-owner channel reference
	ProviderId                string // Provider descriptor identifier declared through resourcecap SPI
	ProviderPluginId          string // Plugin that owns the frozen Provider implementation
	DisplayName               string //
	Endpoint                  string // Administrator-approved HTTPS API origin without credentials
	NonSecretSettings         string // Adapter settings validated to exclude credentials and secret references
	StatusCode                string //
	Version                   string //
	CreatedBy                 string //
	UpdatedBy                 string //
	CreatedAt                 string //
	UpdatedAt                 string //
	DeletedAt                 string // Recoverable configuration deletion time; historical records remain referenced
	ProviderRuntimeType       string // Frozen Provider runtime type: source or dynamic
	ProviderContractVersion   string // Exact owner contract version frozen when a dynamic Provider channel is created
	ProviderReleaseVersion    string // Exact managed plugin release version frozen when a dynamic Provider channel is created
	ProviderReleaseDigest     string // Exact managed plugin release digest frozen when a dynamic Provider channel is created
	ProviderDeclarationDigest string // Exact resource-provider declaration digest frozen when a dynamic Provider channel is created
}

// channelColumns holds the columns for the table plugin_idc_resource_channel.
var channelColumns = ChannelColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	ChannelNo:                 "channel_no",
	ProviderId:                "provider_id",
	ProviderPluginId:          "provider_plugin_id",
	DisplayName:               "display_name",
	Endpoint:                  "endpoint",
	NonSecretSettings:         "non_secret_settings",
	StatusCode:                "status_code",
	Version:                   "version",
	CreatedBy:                 "created_by",
	UpdatedBy:                 "updated_by",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
	DeletedAt:                 "deleted_at",
	ProviderRuntimeType:       "provider_runtime_type",
	ProviderContractVersion:   "provider_contract_version",
	ProviderReleaseVersion:    "provider_release_version",
	ProviderReleaseDigest:     "provider_release_digest",
	ProviderDeclarationDigest: "provider_declaration_digest",
}

// NewChannelDao creates and returns a new DAO object for table data access.
func NewChannelDao(handlers ...gdb.ModelHandler) *ChannelDao {
	return &ChannelDao{
		group:    "default",
		table:    "plugin_idc_resource_channel",
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
