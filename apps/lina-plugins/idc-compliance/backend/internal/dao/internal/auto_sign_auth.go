// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AutoSignAuthDao is the data access object for the table plugin_idc_compliance_auto_sign_auth.
type AutoSignAuthDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  AutoSignAuthColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// AutoSignAuthColumns defines and stores column names for the table plugin_idc_compliance_auto_sign_auth.
type AutoSignAuthColumns struct {
	Id                          string //
	TenantId                    string //
	AuthorizationNo             string //
	ProviderCode                string //
	CurrencyCode                string //
	AllowRequiredOrder          string //
	AllowManualOptional         string //
	MaxAmountMinor              string //
	SuccessLimit                string //
	SuccessCount                string //
	StatusCode                  string //
	ExpiresAt                   string //
	CreatedByUserId             string //
	RevokedByUserId             string //
	RevokedAt                   string //
	CreationIdempotencyDigest   string //
	CreationFingerprint         string //
	RevocationIdempotencyDigest string //
	RevocationFingerprint       string //
	Version                     string //
	CreatedAt                   string //
	UpdatedAt                   string //
}

// autoSignAuthColumns holds the columns for the table plugin_idc_compliance_auto_sign_auth.
var autoSignAuthColumns = AutoSignAuthColumns{
	Id:                          "id",
	TenantId:                    "tenant_id",
	AuthorizationNo:             "authorization_no",
	ProviderCode:                "provider_code",
	CurrencyCode:                "currency_code",
	AllowRequiredOrder:          "allow_required_order",
	AllowManualOptional:         "allow_manual_optional",
	MaxAmountMinor:              "max_amount_minor",
	SuccessLimit:                "success_limit",
	SuccessCount:                "success_count",
	StatusCode:                  "status_code",
	ExpiresAt:                   "expires_at",
	CreatedByUserId:             "created_by_user_id",
	RevokedByUserId:             "revoked_by_user_id",
	RevokedAt:                   "revoked_at",
	CreationIdempotencyDigest:   "creation_idempotency_digest",
	CreationFingerprint:         "creation_fingerprint",
	RevocationIdempotencyDigest: "revocation_idempotency_digest",
	RevocationFingerprint:       "revocation_fingerprint",
	Version:                     "version",
	CreatedAt:                   "created_at",
	UpdatedAt:                   "updated_at",
}

// NewAutoSignAuthDao creates and returns a new DAO object for table data access.
func NewAutoSignAuthDao(handlers ...gdb.ModelHandler) *AutoSignAuthDao {
	return &AutoSignAuthDao{
		group:    "default",
		table:    "plugin_idc_compliance_auto_sign_auth",
		columns:  autoSignAuthColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AutoSignAuthDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AutoSignAuthDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AutoSignAuthDao) Columns() AutoSignAuthColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AutoSignAuthDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AutoSignAuthDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AutoSignAuthDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
