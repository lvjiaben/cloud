// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecyclePolicyDao is the data access object for the table plugin_idc_core_service_lifecycle_policy.
type ServiceLifecyclePolicyDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  ServiceLifecyclePolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// ServiceLifecyclePolicyColumns defines and stores column names for the table plugin_idc_core_service_lifecycle_policy.
type ServiceLifecyclePolicyColumns struct {
	Id                    string //
	TenantId              string //
	PolicyNo              string //
	ServiceNo             string //
	PolicyVersion         string //
	ReminderCount         string //
	GraceSeconds          string //
	SuspendEnabled        string //
	SuspendAfterSeconds   string //
	TerminateEnabled      string //
	TerminateAfterSeconds string //
	ConfiguredBy          string //
	ContentDigest         string //
	CreatedAt             string //
}

// serviceLifecyclePolicyColumns holds the columns for the table plugin_idc_core_service_lifecycle_policy.
var serviceLifecyclePolicyColumns = ServiceLifecyclePolicyColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	PolicyNo:              "policy_no",
	ServiceNo:             "service_no",
	PolicyVersion:         "policy_version",
	ReminderCount:         "reminder_count",
	GraceSeconds:          "grace_seconds",
	SuspendEnabled:        "suspend_enabled",
	SuspendAfterSeconds:   "suspend_after_seconds",
	TerminateEnabled:      "terminate_enabled",
	TerminateAfterSeconds: "terminate_after_seconds",
	ConfiguredBy:          "configured_by",
	ContentDigest:         "content_digest",
	CreatedAt:             "created_at",
}

// NewServiceLifecyclePolicyDao creates and returns a new DAO object for table data access.
func NewServiceLifecyclePolicyDao(handlers ...gdb.ModelHandler) *ServiceLifecyclePolicyDao {
	return &ServiceLifecyclePolicyDao{
		group:    "default",
		table:    "plugin_idc_core_service_lifecycle_policy",
		columns:  serviceLifecyclePolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceLifecyclePolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceLifecyclePolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceLifecyclePolicyDao) Columns() ServiceLifecyclePolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceLifecyclePolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceLifecyclePolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceLifecyclePolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
