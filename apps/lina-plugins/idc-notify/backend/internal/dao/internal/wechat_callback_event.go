// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// WechatCallbackEventDao is the data access object for the table plugin_idc_notify_wechat_callback_event.
type WechatCallbackEventDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  WechatCallbackEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// WechatCallbackEventColumns defines and stores column names for the table plugin_idc_notify_wechat_callback_event.
type WechatCallbackEventColumns struct {
	TenantId           string //
	CallbackNo         string //
	ProviderMessageRef string //
	EventDigest        string //
	ResultClass        string //
	AppliedTaskNo      string //
	ReceivedAt         string //
}

// wechatCallbackEventColumns holds the columns for the table plugin_idc_notify_wechat_callback_event.
var wechatCallbackEventColumns = WechatCallbackEventColumns{
	TenantId:           "tenant_id",
	CallbackNo:         "callback_no",
	ProviderMessageRef: "provider_message_ref",
	EventDigest:        "event_digest",
	ResultClass:        "result_class",
	AppliedTaskNo:      "applied_task_no",
	ReceivedAt:         "received_at",
}

// NewWechatCallbackEventDao creates and returns a new DAO object for table data access.
func NewWechatCallbackEventDao(handlers ...gdb.ModelHandler) *WechatCallbackEventDao {
	return &WechatCallbackEventDao{
		group:    "default",
		table:    "plugin_idc_notify_wechat_callback_event",
		columns:  wechatCallbackEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *WechatCallbackEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *WechatCallbackEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *WechatCallbackEventDao) Columns() WechatCallbackEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *WechatCallbackEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *WechatCallbackEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *WechatCallbackEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
