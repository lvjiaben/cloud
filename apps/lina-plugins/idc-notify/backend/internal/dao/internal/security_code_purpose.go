// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SecurityCodePurposeDao is the data access object for the table plugin_idc_notify_security_code_purpose.
type SecurityCodePurposeDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SecurityCodePurposeColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SecurityCodePurposeColumns defines and stores column names for the table plugin_idc_notify_security_code_purpose.
type SecurityCodePurposeColumns struct {
	TenantId       string //
	CallerPluginId string //
	PurposeCode    string //
	EventCode      string //
	Enabled        string //
	Version        string //
	CreatedAt      string //
	UpdatedAt      string //
}

// securityCodePurposeColumns holds the columns for the table plugin_idc_notify_security_code_purpose.
var securityCodePurposeColumns = SecurityCodePurposeColumns{
	TenantId:       "tenant_id",
	CallerPluginId: "caller_plugin_id",
	PurposeCode:    "purpose_code",
	EventCode:      "event_code",
	Enabled:        "enabled",
	Version:        "version",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewSecurityCodePurposeDao creates and returns a new DAO object for table data access.
func NewSecurityCodePurposeDao(handlers ...gdb.ModelHandler) *SecurityCodePurposeDao {
	return &SecurityCodePurposeDao{
		group:    "default",
		table:    "plugin_idc_notify_security_code_purpose",
		columns:  securityCodePurposeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SecurityCodePurposeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SecurityCodePurposeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SecurityCodePurposeDao) Columns() SecurityCodePurposeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SecurityCodePurposeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SecurityCodePurposeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SecurityCodePurposeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
