// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelOperationDao is the data access object for the table plugin_idc_notify_channel_operation.
type ChannelOperationDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  ChannelOperationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// ChannelOperationColumns defines and stores column names for the table plugin_idc_notify_channel_operation.
type ChannelOperationColumns struct {
	TenantId          string //
	OperationNo       string //
	ChannelCode       string //
	OperationCode     string //
	ActorUserId       string //
	ReasonCode        string //
	BeforeVersion     string //
	AfterVersion      string //
	AffectedTaskCount string //
	CreatedAt         string //
}

// channelOperationColumns holds the columns for the table plugin_idc_notify_channel_operation.
var channelOperationColumns = ChannelOperationColumns{
	TenantId:          "tenant_id",
	OperationNo:       "operation_no",
	ChannelCode:       "channel_code",
	OperationCode:     "operation_code",
	ActorUserId:       "actor_user_id",
	ReasonCode:        "reason_code",
	BeforeVersion:     "before_version",
	AfterVersion:      "after_version",
	AffectedTaskCount: "affected_task_count",
	CreatedAt:         "created_at",
}

// NewChannelOperationDao creates and returns a new DAO object for table data access.
func NewChannelOperationDao(handlers ...gdb.ModelHandler) *ChannelOperationDao {
	return &ChannelOperationDao{
		group:    "default",
		table:    "plugin_idc_notify_channel_operation",
		columns:  channelOperationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelOperationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelOperationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelOperationDao) Columns() ChannelOperationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelOperationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelOperationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelOperationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
