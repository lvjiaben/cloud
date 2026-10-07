// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractSignerIdentityAttemptDao is the data access object for the table plugin_idc_compliance_contract_signer_identity_attempt.
type ContractSignerIdentityAttemptDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  ContractSignerIdentityAttemptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// ContractSignerIdentityAttemptColumns defines and stores column names for the table plugin_idc_compliance_contract_signer_identity_attempt.
type ContractSignerIdentityAttemptColumns struct {
	Id                      string //
	TenantId                string //
	AttemptNo               string //
	SignerNo                string //
	OuterOrderNo            string //
	ConfigurationNo         string //
	ConfigurationVersion    string //
	CredentialGeneration    string //
	CertifyId               string //
	StatusCode              string //
	ReasonCode              string //
	InitializationStartedAt string //
	CertifyExpiresAt        string //
	LastQueriedAt           string //
	PassedAt                string //
	ExpiredAt               string //
	Version                 string //
	CreatedAt               string //
	UpdatedAt               string //
}

// contractSignerIdentityAttemptColumns holds the columns for the table plugin_idc_compliance_contract_signer_identity_attempt.
var contractSignerIdentityAttemptColumns = ContractSignerIdentityAttemptColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	AttemptNo:               "attempt_no",
	SignerNo:                "signer_no",
	OuterOrderNo:            "outer_order_no",
	ConfigurationNo:         "configuration_no",
	ConfigurationVersion:    "configuration_version",
	CredentialGeneration:    "credential_generation",
	CertifyId:               "certify_id",
	StatusCode:              "status_code",
	ReasonCode:              "reason_code",
	InitializationStartedAt: "initialization_started_at",
	CertifyExpiresAt:        "certify_expires_at",
	LastQueriedAt:           "last_queried_at",
	PassedAt:                "passed_at",
	ExpiredAt:               "expired_at",
	Version:                 "version",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewContractSignerIdentityAttemptDao creates and returns a new DAO object for table data access.
func NewContractSignerIdentityAttemptDao(handlers ...gdb.ModelHandler) *ContractSignerIdentityAttemptDao {
	return &ContractSignerIdentityAttemptDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_signer_identity_attempt",
		columns:  contractSignerIdentityAttemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractSignerIdentityAttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractSignerIdentityAttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractSignerIdentityAttemptDao) Columns() ContractSignerIdentityAttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractSignerIdentityAttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractSignerIdentityAttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractSignerIdentityAttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
