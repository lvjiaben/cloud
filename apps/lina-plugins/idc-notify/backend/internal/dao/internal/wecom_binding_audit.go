// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// WecomBindingAuditDao is the data access object for the table plugin_idc_notify_wecom_binding_audit.
type WecomBindingAuditDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  WecomBindingAuditColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// WecomBindingAuditColumns defines and stores column names for the table plugin_idc_notify_wecom_binding_audit.
type WecomBindingAuditColumns struct {
	TenantId       string //
	AuditNo        string //
	BindingNo      string //
	ActorUserId    string //
	ActionCode     string //
	ReasonCode     string //
	PreviousStatus string //
	CurrentStatus  string //
	BindingVersion string //
	CreatedAt      string //
}

// wecomBindingAuditColumns holds the columns for the table plugin_idc_notify_wecom_binding_audit.
var wecomBindingAuditColumns = WecomBindingAuditColumns{
	TenantId:       "tenant_id",
	AuditNo:        "audit_no",
	BindingNo:      "binding_no",
	ActorUserId:    "actor_user_id",
	ActionCode:     "action_code",
	ReasonCode:     "reason_code",
	PreviousStatus: "previous_status",
	CurrentStatus:  "current_status",
	BindingVersion: "binding_version",
	CreatedAt:      "created_at",
}

// NewWecomBindingAuditDao creates and returns a new DAO object for table data access.
func NewWecomBindingAuditDao(handlers ...gdb.ModelHandler) *WecomBindingAuditDao {
	return &WecomBindingAuditDao{
		group:    "default",
		table:    "plugin_idc_notify_wecom_binding_audit",
		columns:  wecomBindingAuditColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *WecomBindingAuditDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *WecomBindingAuditDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *WecomBindingAuditDao) Columns() WecomBindingAuditColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *WecomBindingAuditDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *WecomBindingAuditDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *WecomBindingAuditDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
