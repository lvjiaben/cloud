// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerRuntimeFactDao is the data access object for the table plugin_idc_resource_server_runtime_fact.
type ServerRuntimeFactDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  ServerRuntimeFactColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// ServerRuntimeFactColumns defines and stores column names for the table plugin_idc_resource_server_runtime_fact.
type ServerRuntimeFactColumns struct {
	MappingNo      string //
	TenantId       string //
	Facts          string //
	ObservedAt     string //
	NextRefreshAt  string //
	LeaseUntil     string //
	LeaseVersion   string //
	LastErrorCode  string //
	MappingVersion string //
}

// serverRuntimeFactColumns holds the columns for the table plugin_idc_resource_server_runtime_fact.
var serverRuntimeFactColumns = ServerRuntimeFactColumns{
	MappingNo:      "mapping_no",
	TenantId:       "tenant_id",
	Facts:          "facts",
	ObservedAt:     "observed_at",
	NextRefreshAt:  "next_refresh_at",
	LeaseUntil:     "lease_until",
	LeaseVersion:   "lease_version",
	LastErrorCode:  "last_error_code",
	MappingVersion: "mapping_version",
}

// NewServerRuntimeFactDao creates and returns a new DAO object for table data access.
func NewServerRuntimeFactDao(handlers ...gdb.ModelHandler) *ServerRuntimeFactDao {
	return &ServerRuntimeFactDao{
		group:    "default",
		table:    "plugin_idc_resource_server_runtime_fact",
		columns:  serverRuntimeFactColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerRuntimeFactDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerRuntimeFactDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerRuntimeFactDao) Columns() ServerRuntimeFactColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerRuntimeFactDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerRuntimeFactDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerRuntimeFactDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
