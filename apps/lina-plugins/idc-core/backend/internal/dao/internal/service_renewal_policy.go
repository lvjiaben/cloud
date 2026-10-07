// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceRenewalPolicyDao is the data access object for the table plugin_idc_core_service_renewal_policy.
type ServiceRenewalPolicyDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ServiceRenewalPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ServiceRenewalPolicyColumns defines and stores column names for the table plugin_idc_core_service_renewal_policy.
type ServiceRenewalPolicyColumns struct {
	Id                         string //
	TenantId                   string //
	PolicyNo                   string //
	EntitlementNo              string //
	PolicyVersion              string //
	AutoRenewEnabled           string //
	RenewalCycles              string //
	LeadSeconds                string //
	MaximumAttempts            string //
	RetryIntervalSeconds       string //
	FailureNotificationEnabled string //
	ConfiguredBy               string //
	ContentDigest              string //
	CreatedAt                  string //
}

// serviceRenewalPolicyColumns holds the columns for the table plugin_idc_core_service_renewal_policy.
var serviceRenewalPolicyColumns = ServiceRenewalPolicyColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	PolicyNo:                   "policy_no",
	EntitlementNo:              "entitlement_no",
	PolicyVersion:              "policy_version",
	AutoRenewEnabled:           "auto_renew_enabled",
	RenewalCycles:              "renewal_cycles",
	LeadSeconds:                "lead_seconds",
	MaximumAttempts:            "maximum_attempts",
	RetryIntervalSeconds:       "retry_interval_seconds",
	FailureNotificationEnabled: "failure_notification_enabled",
	ConfiguredBy:               "configured_by",
	ContentDigest:              "content_digest",
	CreatedAt:                  "created_at",
}

// NewServiceRenewalPolicyDao creates and returns a new DAO object for table data access.
func NewServiceRenewalPolicyDao(handlers ...gdb.ModelHandler) *ServiceRenewalPolicyDao {
	return &ServiceRenewalPolicyDao{
		group:    "default",
		table:    "plugin_idc_core_service_renewal_policy",
		columns:  serviceRenewalPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceRenewalPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceRenewalPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceRenewalPolicyDao) Columns() ServiceRenewalPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceRenewalPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceRenewalPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceRenewalPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
