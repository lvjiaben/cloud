// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecycleReminderRuleDao is the data access object for the table plugin_idc_core_service_lifecycle_reminder_rule.
type ServiceLifecycleReminderRuleDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  ServiceLifecycleReminderRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// ServiceLifecycleReminderRuleColumns defines and stores column names for the table plugin_idc_core_service_lifecycle_reminder_rule.
type ServiceLifecycleReminderRuleColumns struct {
	Id               string //
	TenantId         string //
	ReminderRuleNo   string //
	PolicyNo         string //
	PolicyVersion    string //
	ServiceNo        string //
	ReminderSequence string //
	RelativeSeconds  string //
	ContentDigest    string //
	CreatedAt        string //
}

// serviceLifecycleReminderRuleColumns holds the columns for the table plugin_idc_core_service_lifecycle_reminder_rule.
var serviceLifecycleReminderRuleColumns = ServiceLifecycleReminderRuleColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	ReminderRuleNo:   "reminder_rule_no",
	PolicyNo:         "policy_no",
	PolicyVersion:    "policy_version",
	ServiceNo:        "service_no",
	ReminderSequence: "reminder_sequence",
	RelativeSeconds:  "relative_seconds",
	ContentDigest:    "content_digest",
	CreatedAt:        "created_at",
}

// NewServiceLifecycleReminderRuleDao creates and returns a new DAO object for table data access.
func NewServiceLifecycleReminderRuleDao(handlers ...gdb.ModelHandler) *ServiceLifecycleReminderRuleDao {
	return &ServiceLifecycleReminderRuleDao{
		group:    "default",
		table:    "plugin_idc_core_service_lifecycle_reminder_rule",
		columns:  serviceLifecycleReminderRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceLifecycleReminderRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceLifecycleReminderRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceLifecycleReminderRuleDao) Columns() ServiceLifecycleReminderRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceLifecycleReminderRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceLifecycleReminderRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceLifecycleReminderRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
