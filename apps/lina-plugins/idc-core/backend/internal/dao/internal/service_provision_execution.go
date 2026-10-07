// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceProvisionExecutionDao is the data access object for the table plugin_idc_core_service_provision_execution.
type ServiceProvisionExecutionDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  ServiceProvisionExecutionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// ServiceProvisionExecutionColumns defines and stores column names for the table plugin_idc_core_service_provision_execution.
type ServiceProvisionExecutionColumns struct {
	Id                       string //
	TenantId                 string //
	ExecutionNo              string //
	OperationNo              string //
	ServiceNo                string //
	ResourceOwnerCode        string //
	ProviderTaskNo           string // Opaque resource-owner task reference; never a Provider task identifier
	ChannelNo                string //
	StatusCode               string //
	RequestDigest            string //
	CapabilityEvidenceNo     string //
	CapabilityEvidenceDigest string //
	TaskEvidenceDigest       string //
	LastFailureCode          string //
	OutcomeReportStatus      string //
	OutcomeReportedAt        string //
	ObservedAt               string //
	CompletedAt              string //
	Version                  string //
	CreatedAt                string //
	UpdatedAt                string //
}

// serviceProvisionExecutionColumns holds the columns for the table plugin_idc_core_service_provision_execution.
var serviceProvisionExecutionColumns = ServiceProvisionExecutionColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	ExecutionNo:              "execution_no",
	OperationNo:              "operation_no",
	ServiceNo:                "service_no",
	ResourceOwnerCode:        "resource_owner_code",
	ProviderTaskNo:           "provider_task_no",
	ChannelNo:                "channel_no",
	StatusCode:               "status_code",
	RequestDigest:            "request_digest",
	CapabilityEvidenceNo:     "capability_evidence_no",
	CapabilityEvidenceDigest: "capability_evidence_digest",
	TaskEvidenceDigest:       "task_evidence_digest",
	LastFailureCode:          "last_failure_code",
	OutcomeReportStatus:      "outcome_report_status",
	OutcomeReportedAt:        "outcome_reported_at",
	ObservedAt:               "observed_at",
	CompletedAt:              "completed_at",
	Version:                  "version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewServiceProvisionExecutionDao creates and returns a new DAO object for table data access.
func NewServiceProvisionExecutionDao(handlers ...gdb.ModelHandler) *ServiceProvisionExecutionDao {
	return &ServiceProvisionExecutionDao{
		group:    "default",
		table:    "plugin_idc_core_service_provision_execution",
		columns:  serviceProvisionExecutionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceProvisionExecutionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceProvisionExecutionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceProvisionExecutionDao) Columns() ServiceProvisionExecutionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceProvisionExecutionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceProvisionExecutionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceProvisionExecutionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
