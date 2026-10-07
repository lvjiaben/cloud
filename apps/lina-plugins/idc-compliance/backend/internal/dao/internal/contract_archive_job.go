// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractArchiveJobDao is the data access object for the table plugin_idc_compliance_contract_archive_job.
type ContractArchiveJobDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  ContractArchiveJobColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// ContractArchiveJobColumns defines and stores column names for the table plugin_idc_compliance_contract_archive_job.
type ContractArchiveJobColumns struct {
	Id                 string //
	TenantId           string //
	JobNo              string //
	ContractNo         string //
	FlowNo             string //
	GatewayRouteNo     string //
	ArtifactTypeCode   string //
	StatusCode         string //
	AttemptCount       string //
	NextAttemptAt      string //
	LeaseSha256        string //
	LeaseExpiresAt     string //
	LastReasonCode     string //
	SupplierObservedAt string //
	CompletedAt        string //
	CreatedAt          string //
	UpdatedAt          string //
	Version            string //
}

// contractArchiveJobColumns holds the columns for the table plugin_idc_compliance_contract_archive_job.
var contractArchiveJobColumns = ContractArchiveJobColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	JobNo:              "job_no",
	ContractNo:         "contract_no",
	FlowNo:             "flow_no",
	GatewayRouteNo:     "gateway_route_no",
	ArtifactTypeCode:   "artifact_type_code",
	StatusCode:         "status_code",
	AttemptCount:       "attempt_count",
	NextAttemptAt:      "next_attempt_at",
	LeaseSha256:        "lease_sha256",
	LeaseExpiresAt:     "lease_expires_at",
	LastReasonCode:     "last_reason_code",
	SupplierObservedAt: "supplier_observed_at",
	CompletedAt:        "completed_at",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
	Version:            "version",
}

// NewContractArchiveJobDao creates and returns a new DAO object for table data access.
func NewContractArchiveJobDao(handlers ...gdb.ModelHandler) *ContractArchiveJobDao {
	return &ContractArchiveJobDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_archive_job",
		columns:  contractArchiveJobColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractArchiveJobDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractArchiveJobDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractArchiveJobDao) Columns() ContractArchiveJobColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractArchiveJobDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractArchiveJobDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractArchiveJobDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
