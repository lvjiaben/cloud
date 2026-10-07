// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrincipalTaskDao is the data access object for the table plugin_idc_resource_principal_task.
type PrincipalTaskDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  PrincipalTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// PrincipalTaskColumns defines and stores column names for the table plugin_idc_resource_principal_task.
type PrincipalTaskColumns struct {
	Id                           string //
	TenantId                     string //
	TaskNo                       string //
	CustomerNo                   string //
	ChannelNo                    string //
	UsernameSecretRef            string // Channel-scoped secretcap handle; never a username or customer profile field
	PasswordSecretRef            string // Channel-scoped secretcap handle; plaintext is never persisted in the resource schema
	StatusCode                   string //
	ExpectedExternalPrincipalRef string //
	ResultUnknown                string // Hard replay fence: when true, only exact lookup or manual review is allowed
	AttemptCount                 string // Count of write-eligible claims; unknown writes are never made claimable again
	ObservationAttemptCount      string // Read-only exact lookup attempts; never authorizes another create
	AvailableAt                  string //
	LeaseTokenDigest             string //
	LeaseExpiresAt               string //
	FailureCode                  string //
	EvidenceDigest               string //
	FinishedAt                   string // Confirmed mappings revoke both handles; manual review revokes the password and retains only its encrypted username handle for operator reconciliation
	Version                      string //
	CreatedAt                    string //
	UpdatedAt                    string //
}

// principalTaskColumns holds the columns for the table plugin_idc_resource_principal_task.
var principalTaskColumns = PrincipalTaskColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	TaskNo:                       "task_no",
	CustomerNo:                   "customer_no",
	ChannelNo:                    "channel_no",
	UsernameSecretRef:            "username_secret_ref",
	PasswordSecretRef:            "password_secret_ref",
	StatusCode:                   "status_code",
	ExpectedExternalPrincipalRef: "expected_external_principal_ref",
	ResultUnknown:                "result_unknown",
	AttemptCount:                 "attempt_count",
	ObservationAttemptCount:      "observation_attempt_count",
	AvailableAt:                  "available_at",
	LeaseTokenDigest:             "lease_token_digest",
	LeaseExpiresAt:               "lease_expires_at",
	FailureCode:                  "failure_code",
	EvidenceDigest:               "evidence_digest",
	FinishedAt:                   "finished_at",
	Version:                      "version",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
}

// NewPrincipalTaskDao creates and returns a new DAO object for table data access.
func NewPrincipalTaskDao(handlers ...gdb.ModelHandler) *PrincipalTaskDao {
	return &PrincipalTaskDao{
		group:    "default",
		table:    "plugin_idc_resource_principal_task",
		columns:  principalTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrincipalTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrincipalTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrincipalTaskDao) Columns() PrincipalTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrincipalTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrincipalTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrincipalTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
