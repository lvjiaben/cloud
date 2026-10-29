// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerOperationPolicyDao is the data access object for the table plugin_idc_core_server_operation_policy.
type ServerOperationPolicyDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  ServerOperationPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// ServerOperationPolicyColumns defines and stores column names for the table plugin_idc_core_server_operation_policy.
type ServerOperationPolicyColumns struct {
	TenantId              string //
	RequireHighRiskReauth string //
	Version               string //
	CreatedAt             string //
	UpdatedAt             string //
	AllowCustomerTransfer string //
}

// serverOperationPolicyColumns holds the columns for the table plugin_idc_core_server_operation_policy.
var serverOperationPolicyColumns = ServerOperationPolicyColumns{
	TenantId:              "tenant_id",
	RequireHighRiskReauth: "require_high_risk_reauth",
	Version:               "version",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	AllowCustomerTransfer: "allow_customer_transfer",
}

// NewServerOperationPolicyDao creates and returns a new DAO object for table data access.
func NewServerOperationPolicyDao(handlers ...gdb.ModelHandler) *ServerOperationPolicyDao {
	return &ServerOperationPolicyDao{
		group:    "default",
		table:    "plugin_idc_core_server_operation_policy",
		columns:  serverOperationPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerOperationPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerOperationPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerOperationPolicyDao) Columns() ServerOperationPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerOperationPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerOperationPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerOperationPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
