// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProviderApiLogDao is the data access object for the table plugin_idc_resource_provider_api_log.
type ProviderApiLogDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  ProviderApiLogColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// ProviderApiLogColumns defines and stores column names for the table plugin_idc_resource_provider_api_log.
type ProviderApiLogColumns struct {
	Id               string //
	TenantId         string //
	ChannelNo        string //
	ProviderPluginId string //
	MethodCode       string //
	Succeeded        string //
	ErrorCode        string //
	DurationMs       string //
	StartedAt        string //
	CreatedAt        string //
}

// providerApiLogColumns holds the columns for the table plugin_idc_resource_provider_api_log.
var providerApiLogColumns = ProviderApiLogColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	ChannelNo:        "channel_no",
	ProviderPluginId: "provider_plugin_id",
	MethodCode:       "method_code",
	Succeeded:        "succeeded",
	ErrorCode:        "error_code",
	DurationMs:       "duration_ms",
	StartedAt:        "started_at",
	CreatedAt:        "created_at",
}

// NewProviderApiLogDao creates and returns a new DAO object for table data access.
func NewProviderApiLogDao(handlers ...gdb.ModelHandler) *ProviderApiLogDao {
	return &ProviderApiLogDao{
		group:    "default",
		table:    "plugin_idc_resource_provider_api_log",
		columns:  providerApiLogColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProviderApiLogDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProviderApiLogDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProviderApiLogDao) Columns() ProviderApiLogColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProviderApiLogDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProviderApiLogDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProviderApiLogDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
