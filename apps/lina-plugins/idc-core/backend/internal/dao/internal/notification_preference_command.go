// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NotificationPreferenceCommandDao is the data access object for the table plugin_idc_core_notification_preference_command.
type NotificationPreferenceCommandDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  NotificationPreferenceCommandColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// NotificationPreferenceCommandColumns defines and stores column names for the table plugin_idc_core_notification_preference_command.
type NotificationPreferenceCommandColumns struct {
	Id                 string //
	TenantId           string //
	CommandNo          string //
	CustomerNo         string //
	IdempotencyDigest  string //
	RequestFingerprint string //
	EvidenceDigest     string //
	ResultVersion      string //
	AppliedAt          string //
}

// notificationPreferenceCommandColumns holds the columns for the table plugin_idc_core_notification_preference_command.
var notificationPreferenceCommandColumns = NotificationPreferenceCommandColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	CommandNo:          "command_no",
	CustomerNo:         "customer_no",
	IdempotencyDigest:  "idempotency_digest",
	RequestFingerprint: "request_fingerprint",
	EvidenceDigest:     "evidence_digest",
	ResultVersion:      "result_version",
	AppliedAt:          "applied_at",
}

// NewNotificationPreferenceCommandDao creates and returns a new DAO object for table data access.
func NewNotificationPreferenceCommandDao(handlers ...gdb.ModelHandler) *NotificationPreferenceCommandDao {
	return &NotificationPreferenceCommandDao{
		group:    "default",
		table:    "plugin_idc_core_notification_preference_command",
		columns:  notificationPreferenceCommandColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NotificationPreferenceCommandDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NotificationPreferenceCommandDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NotificationPreferenceCommandDao) Columns() NotificationPreferenceCommandColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NotificationPreferenceCommandDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NotificationPreferenceCommandDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NotificationPreferenceCommandDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
