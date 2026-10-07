// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentActionHistoryDao is the data access object for the table plugin_idc_core_content_action_history.
type ContentActionHistoryDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ContentActionHistoryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ContentActionHistoryColumns defines and stores column names for the table plugin_idc_core_content_action_history.
type ContentActionHistoryColumns struct {
	Id             string //
	TenantId       string //
	ActionNo       string //
	ContentNo      string //
	ActionCode     string //
	ActorUserId    string //
	ContentVersion string //
	OccurredAt     string //
}

// contentActionHistoryColumns holds the columns for the table plugin_idc_core_content_action_history.
var contentActionHistoryColumns = ContentActionHistoryColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	ActionNo:       "action_no",
	ContentNo:      "content_no",
	ActionCode:     "action_code",
	ActorUserId:    "actor_user_id",
	ContentVersion: "content_version",
	OccurredAt:     "occurred_at",
}

// NewContentActionHistoryDao creates and returns a new DAO object for table data access.
func NewContentActionHistoryDao(handlers ...gdb.ModelHandler) *ContentActionHistoryDao {
	return &ContentActionHistoryDao{
		group:    "default",
		table:    "plugin_idc_core_content_action_history",
		columns:  contentActionHistoryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentActionHistoryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentActionHistoryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentActionHistoryDao) Columns() ContentActionHistoryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentActionHistoryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentActionHistoryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentActionHistoryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
