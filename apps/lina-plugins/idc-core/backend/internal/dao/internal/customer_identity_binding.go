// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerIdentityBindingDao is the data access object for the table plugin_idc_core_customer_identity_binding.
type CustomerIdentityBindingDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  CustomerIdentityBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// CustomerIdentityBindingColumns defines and stores column names for the table plugin_idc_core_customer_identity_binding.
type CustomerIdentityBindingColumns struct {
	Id                   string //
	TenantId             string //
	BindingNo            string //
	SysUserId            string // Host sys_user identity ID validated through host capabilities
	CustomerNo           string // Bound customer business number
	SourceCode           string // Binding source: onboarding, admin invitation, or migration
	SourceReferenceNo    string // Opaque onboarding, invitation, or migration reference
	StatusCode           string //
	ActivatedAt          string //
	RevokedAt            string //
	Version              string //
	CreatedBy            string //
	UpdatedBy            string //
	CreatedAt            string //
	UpdatedAt            string //
	MustChangePassword   string // Direct-created customer must replace the administrator-provided initial password before using account APIs
	IdempotencyKeyDigest string // Irreversible digest of the administrator direct-create retry key
	RequestFingerprint   string // Normalized direct-create request fingerprint excluding plaintext password
}

// customerIdentityBindingColumns holds the columns for the table plugin_idc_core_customer_identity_binding.
var customerIdentityBindingColumns = CustomerIdentityBindingColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	BindingNo:            "binding_no",
	SysUserId:            "sys_user_id",
	CustomerNo:           "customer_no",
	SourceCode:           "source_code",
	SourceReferenceNo:    "source_reference_no",
	StatusCode:           "status_code",
	ActivatedAt:          "activated_at",
	RevokedAt:            "revoked_at",
	Version:              "version",
	CreatedBy:            "created_by",
	UpdatedBy:            "updated_by",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
	MustChangePassword:   "must_change_password",
	IdempotencyKeyDigest: "idempotency_key_digest",
	RequestFingerprint:   "request_fingerprint",
}

// NewCustomerIdentityBindingDao creates and returns a new DAO object for table data access.
func NewCustomerIdentityBindingDao(handlers ...gdb.ModelHandler) *CustomerIdentityBindingDao {
	return &CustomerIdentityBindingDao{
		group:    "default",
		table:    "plugin_idc_core_customer_identity_binding",
		columns:  customerIdentityBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerIdentityBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerIdentityBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerIdentityBindingDao) Columns() CustomerIdentityBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerIdentityBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerIdentityBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerIdentityBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
