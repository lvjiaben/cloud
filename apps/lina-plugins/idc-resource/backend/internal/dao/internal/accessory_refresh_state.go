// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AccessoryRefreshStateDao is the data access object for the table plugin_idc_resource_accessory_refresh_state.
type AccessoryRefreshStateDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  AccessoryRefreshStateColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// AccessoryRefreshStateColumns defines and stores column names for the table plugin_idc_resource_accessory_refresh_state.
type AccessoryRefreshStateColumns struct {
	Id               string //
	TenantId         string //
	MappingNo        string //
	NextAttemptAt    string //
	AttemptCount     string //
	LeaseTokenDigest string //
	LeaseExpiresAt   string //
	FailureCode      string //
	UpdatedAt        string //
}

// accessoryRefreshStateColumns holds the columns for the table plugin_idc_resource_accessory_refresh_state.
var accessoryRefreshStateColumns = AccessoryRefreshStateColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	MappingNo:        "mapping_no",
	NextAttemptAt:    "next_attempt_at",
	AttemptCount:     "attempt_count",
	LeaseTokenDigest: "lease_token_digest",
	LeaseExpiresAt:   "lease_expires_at",
	FailureCode:      "failure_code",
	UpdatedAt:        "updated_at",
}

// NewAccessoryRefreshStateDao creates and returns a new DAO object for table data access.
func NewAccessoryRefreshStateDao(handlers ...gdb.ModelHandler) *AccessoryRefreshStateDao {
	return &AccessoryRefreshStateDao{
		group:    "default",
		table:    "plugin_idc_resource_accessory_refresh_state",
		columns:  accessoryRefreshStateColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AccessoryRefreshStateDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AccessoryRefreshStateDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AccessoryRefreshStateDao) Columns() AccessoryRefreshStateColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AccessoryRefreshStateDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AccessoryRefreshStateDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AccessoryRefreshStateDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
