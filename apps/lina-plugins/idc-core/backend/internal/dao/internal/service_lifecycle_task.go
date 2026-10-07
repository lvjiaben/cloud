// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecycleTaskDao is the data access object for the table plugin_idc_core_service_lifecycle_task.
type ServiceLifecycleTaskDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ServiceLifecycleTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ServiceLifecycleTaskColumns defines and stores column names for the table plugin_idc_core_service_lifecycle_task.
type ServiceLifecycleTaskColumns struct {
	Id                           string //
	TenantId                     string //
	TaskNo                       string //
	ServiceNo                    string //
	CustomerNo                   string //
	CurrencyCode                 string //
	PolicyNo                     string //
	PolicyVersion                string //
	ActionCode                   string //
	ReminderSequence             string //
	ExpectedServiceVersion       string //
	ExpectedEntitlementExpiresAt string //
	DueAt                        string //
	StatusCode                   string //
	BusinessOperationNo          string //
	PreviousRuntimeStatusCode    string //
	LastErrorCode                string //
	IdempotencyDigest            string //
	ContentDigest                string //
	ClaimedAt                    string //
	DispatchedAt                 string //
	CompletedAt                  string //
	Version                      string //
	CreatedAt                    string //
	UpdatedAt                    string //
	ResidualResources            string // At most 16 bounded non-secret resource kind, opaque number, safe status and stable reason projections that still require reclamation
}

// serviceLifecycleTaskColumns holds the columns for the table plugin_idc_core_service_lifecycle_task.
var serviceLifecycleTaskColumns = ServiceLifecycleTaskColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	TaskNo:                       "task_no",
	ServiceNo:                    "service_no",
	CustomerNo:                   "customer_no",
	CurrencyCode:                 "currency_code",
	PolicyNo:                     "policy_no",
	PolicyVersion:                "policy_version",
	ActionCode:                   "action_code",
	ReminderSequence:             "reminder_sequence",
	ExpectedServiceVersion:       "expected_service_version",
	ExpectedEntitlementExpiresAt: "expected_entitlement_expires_at",
	DueAt:                        "due_at",
	StatusCode:                   "status_code",
	BusinessOperationNo:          "business_operation_no",
	PreviousRuntimeStatusCode:    "previous_runtime_status_code",
	LastErrorCode:                "last_error_code",
	IdempotencyDigest:            "idempotency_digest",
	ContentDigest:                "content_digest",
	ClaimedAt:                    "claimed_at",
	DispatchedAt:                 "dispatched_at",
	CompletedAt:                  "completed_at",
	Version:                      "version",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
	ResidualResources:            "residual_resources",
}

// NewServiceLifecycleTaskDao creates and returns a new DAO object for table data access.
func NewServiceLifecycleTaskDao(handlers ...gdb.ModelHandler) *ServiceLifecycleTaskDao {
	return &ServiceLifecycleTaskDao{
		group:    "default",
		table:    "plugin_idc_core_service_lifecycle_task",
		columns:  serviceLifecycleTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceLifecycleTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceLifecycleTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceLifecycleTaskDao) Columns() ServiceLifecycleTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceLifecycleTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceLifecycleTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceLifecycleTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
