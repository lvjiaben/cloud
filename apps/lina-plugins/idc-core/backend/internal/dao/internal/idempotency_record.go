// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// IdempotencyRecordDao is the data access object for the table plugin_idc_core_idempotency_record.
type IdempotencyRecordDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  IdempotencyRecordColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// IdempotencyRecordColumns defines and stores column names for the table plugin_idc_core_idempotency_record.
type IdempotencyRecordColumns struct {
	Id                   string //
	TenantId             string //
	Scope                string // Stable command or endpoint idempotency namespace
	CallerScopeDigest    string // Digest of the normalized customer, staff, channel, or system caller scope
	IdempotencyKeyDigest string // SHA-256 digest of the caller idempotency key
	RequestFingerprint   string // SHA-256 digest of canonical request semantics
	StatusCode           string //
	LockOwner            string //
	LockExpiresAt        string //
	ResourceType         string //
	ResourceNo           string //
	ResponseStatusCode   string //
	ResponseSnapshot     string // Redacted normalized response used for safe replay
	ExpiresAt            string // Earliest time at which retention cleanup may evaluate the record
	CompletedAt          string //
	CreatedAt            string //
	UpdatedAt            string //
}

// idempotencyRecordColumns holds the columns for the table plugin_idc_core_idempotency_record.
var idempotencyRecordColumns = IdempotencyRecordColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	Scope:                "scope",
	CallerScopeDigest:    "caller_scope_digest",
	IdempotencyKeyDigest: "idempotency_key_digest",
	RequestFingerprint:   "request_fingerprint",
	StatusCode:           "status_code",
	LockOwner:            "lock_owner",
	LockExpiresAt:        "lock_expires_at",
	ResourceType:         "resource_type",
	ResourceNo:           "resource_no",
	ResponseStatusCode:   "response_status_code",
	ResponseSnapshot:     "response_snapshot",
	ExpiresAt:            "expires_at",
	CompletedAt:          "completed_at",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewIdempotencyRecordDao creates and returns a new DAO object for table data access.
func NewIdempotencyRecordDao(handlers ...gdb.ModelHandler) *IdempotencyRecordDao {
	return &IdempotencyRecordDao{
		group:    "default",
		table:    "plugin_idc_core_idempotency_record",
		columns:  idempotencyRecordColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *IdempotencyRecordDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *IdempotencyRecordDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *IdempotencyRecordDao) Columns() IdempotencyRecordColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *IdempotencyRecordDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *IdempotencyRecordDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *IdempotencyRecordDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
