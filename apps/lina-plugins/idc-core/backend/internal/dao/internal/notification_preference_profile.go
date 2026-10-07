// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NotificationPreferenceProfileDao is the data access object for the table plugin_idc_core_notification_preference_profile.
type NotificationPreferenceProfileDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  NotificationPreferenceProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// NotificationPreferenceProfileColumns defines and stores column names for the table plugin_idc_core_notification_preference_profile.
type NotificationPreferenceProfileColumns struct {
	Id         string //
	TenantId   string //
	CustomerNo string //
	Version    string //
	UpdatedAt  string //
}

// notificationPreferenceProfileColumns holds the columns for the table plugin_idc_core_notification_preference_profile.
var notificationPreferenceProfileColumns = NotificationPreferenceProfileColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	CustomerNo: "customer_no",
	Version:    "version",
	UpdatedAt:  "updated_at",
}

// NewNotificationPreferenceProfileDao creates and returns a new DAO object for table data access.
func NewNotificationPreferenceProfileDao(handlers ...gdb.ModelHandler) *NotificationPreferenceProfileDao {
	return &NotificationPreferenceProfileDao{
		group:    "default",
		table:    "plugin_idc_core_notification_preference_profile",
		columns:  notificationPreferenceProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NotificationPreferenceProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NotificationPreferenceProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NotificationPreferenceProfileDao) Columns() NotificationPreferenceProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NotificationPreferenceProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NotificationPreferenceProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NotificationPreferenceProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
