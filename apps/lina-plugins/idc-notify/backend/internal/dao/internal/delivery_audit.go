// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryAuditDao is the data access object for the table plugin_idc_notify_delivery_audit.
type DeliveryAuditDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  DeliveryAuditColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// DeliveryAuditColumns defines and stores column names for the table plugin_idc_notify_delivery_audit.
type DeliveryAuditColumns struct {
	TenantId       string //
	AuditNo        string //
	TaskNo         string //
	ActionCode     string //
	ActorType      string //
	ActorUserId    string //
	ReasonCode     string //
	PreviousStatus string //
	CurrentStatus  string //
	TaskVersion    string //
	CreatedAt      string //
}

// deliveryAuditColumns holds the columns for the table plugin_idc_notify_delivery_audit.
var deliveryAuditColumns = DeliveryAuditColumns{
	TenantId:       "tenant_id",
	AuditNo:        "audit_no",
	TaskNo:         "task_no",
	ActionCode:     "action_code",
	ActorType:      "actor_type",
	ActorUserId:    "actor_user_id",
	ReasonCode:     "reason_code",
	PreviousStatus: "previous_status",
	CurrentStatus:  "current_status",
	TaskVersion:    "task_version",
	CreatedAt:      "created_at",
}

// NewDeliveryAuditDao creates and returns a new DAO object for table data access.
func NewDeliveryAuditDao(handlers ...gdb.ModelHandler) *DeliveryAuditDao {
	return &DeliveryAuditDao{
		group:    "default",
		table:    "plugin_idc_notify_delivery_audit",
		columns:  deliveryAuditColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *DeliveryAuditDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *DeliveryAuditDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *DeliveryAuditDao) Columns() DeliveryAuditColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *DeliveryAuditDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *DeliveryAuditDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *DeliveryAuditDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
