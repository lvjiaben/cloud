// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// EventPolicyDao is the data access object for the table plugin_idc_notify_event_policy.
type EventPolicyDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  EventPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// EventPolicyColumns defines and stores column names for the table plugin_idc_notify_event_policy.
type EventPolicyColumns struct {
	TenantId      string //
	EventCode     string //
	RecipientKind string //
	ChannelCode   string //
	Enabled       string //
	Version       string //
	UpdatedBy     string //
	CreatedAt     string //
	UpdatedAt     string //
}

// eventPolicyColumns holds the columns for the table plugin_idc_notify_event_policy.
var eventPolicyColumns = EventPolicyColumns{
	TenantId:      "tenant_id",
	EventCode:     "event_code",
	RecipientKind: "recipient_kind",
	ChannelCode:   "channel_code",
	Enabled:       "enabled",
	Version:       "version",
	UpdatedBy:     "updated_by",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewEventPolicyDao creates and returns a new DAO object for table data access.
func NewEventPolicyDao(handlers ...gdb.ModelHandler) *EventPolicyDao {
	return &EventPolicyDao{
		group:    "default",
		table:    "plugin_idc_notify_event_policy",
		columns:  eventPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *EventPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *EventPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *EventPolicyDao) Columns() EventPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *EventPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *EventPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *EventPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
