// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProvisionNonAcceptanceVerificationDao is the data access object for the table plugin_idc_resource_provision_non_acceptance_verification.
type ProvisionNonAcceptanceVerificationDao struct {
	table    string                                    // table is the underlying table name of the DAO.
	group    string                                    // group is the database configuration group name of the current DAO.
	columns  ProvisionNonAcceptanceVerificationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                        // handlers for customized model modification.
}

// ProvisionNonAcceptanceVerificationColumns defines and stores column names for the table plugin_idc_resource_provision_non_acceptance_verification.
type ProvisionNonAcceptanceVerificationColumns struct {
	TenantId           string //
	TaskNo             string //
	RequestDigest      string //
	VerificationDigest string //
	ActorUserId        string //
	SafeNote           string //
	VerifiedAt         string //
}

// provisionNonAcceptanceVerificationColumns holds the columns for the table plugin_idc_resource_provision_non_acceptance_verification.
var provisionNonAcceptanceVerificationColumns = ProvisionNonAcceptanceVerificationColumns{
	TenantId:           "tenant_id",
	TaskNo:             "task_no",
	RequestDigest:      "request_digest",
	VerificationDigest: "verification_digest",
	ActorUserId:        "actor_user_id",
	SafeNote:           "safe_note",
	VerifiedAt:         "verified_at",
}

// NewProvisionNonAcceptanceVerificationDao creates and returns a new DAO object for table data access.
func NewProvisionNonAcceptanceVerificationDao(handlers ...gdb.ModelHandler) *ProvisionNonAcceptanceVerificationDao {
	return &ProvisionNonAcceptanceVerificationDao{
		group:    "default",
		table:    "plugin_idc_resource_provision_non_acceptance_verification",
		columns:  provisionNonAcceptanceVerificationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProvisionNonAcceptanceVerificationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProvisionNonAcceptanceVerificationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProvisionNonAcceptanceVerificationDao) Columns() ProvisionNonAcceptanceVerificationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProvisionNonAcceptanceVerificationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProvisionNonAcceptanceVerificationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProvisionNonAcceptanceVerificationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
