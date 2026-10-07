// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// EventMaterializationDao is the data access object for the table plugin_idc_notify_event_materialization.
type EventMaterializationDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  EventMaterializationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// EventMaterializationColumns defines and stores column names for the table plugin_idc_notify_event_materialization.
type EventMaterializationColumns struct {
	TenantId          string //
	MaterializationNo string //
	SourcePlugin      string //
	SourceEventNo     string //
	EventCode         string //
	RecipientKind     string //
	RecipientDigest   string //
	ChannelCode       string //
	PolicyVersion     string //
	StatusCode        string //
	ReasonCode        string //
	TaskNo            string //
	CreatedAt         string //
	UpdatedAt         string //
}

// eventMaterializationColumns holds the columns for the table plugin_idc_notify_event_materialization.
var eventMaterializationColumns = EventMaterializationColumns{
	TenantId:          "tenant_id",
	MaterializationNo: "materialization_no",
	SourcePlugin:      "source_plugin",
	SourceEventNo:     "source_event_no",
	EventCode:         "event_code",
	RecipientKind:     "recipient_kind",
	RecipientDigest:   "recipient_digest",
	ChannelCode:       "channel_code",
	PolicyVersion:     "policy_version",
	StatusCode:        "status_code",
	ReasonCode:        "reason_code",
	TaskNo:            "task_no",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewEventMaterializationDao creates and returns a new DAO object for table data access.
func NewEventMaterializationDao(handlers ...gdb.ModelHandler) *EventMaterializationDao {
	return &EventMaterializationDao{
		group:    "default",
		table:    "plugin_idc_notify_event_materialization",
		columns:  eventMaterializationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *EventMaterializationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *EventMaterializationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *EventMaterializationDao) Columns() EventMaterializationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *EventMaterializationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *EventMaterializationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *EventMaterializationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
