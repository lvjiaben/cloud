// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SecretVersionDao is the data access object for the table plugin_idc_core_secret_version.
type SecretVersionDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SecretVersionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SecretVersionColumns defines and stores column names for the table plugin_idc_core_secret_version.
type SecretVersionColumns struct {
	Id                       string //
	TenantId                 string //
	SecretNo                 string //
	Version                  string //
	EnvelopeVersion          string // Canonical envelope and AAD format version
	CipherAlgorithm          string //
	PayloadNonce             string // Canonical hexadecimal encoding of the random 96-bit AES-GCM payload nonce
	Ciphertext               string // Canonical hexadecimal authenticated ciphertext containing no plaintext key material
	RequestFingerprint       string // DEK-keyed HMAC digest used to reject changed plaintext under one idempotency key
	IdempotencyKeyDigest     string // SHA-256 digest of the caller-supplied idempotency key
	RequestOperation         string // Authenticated write operation discriminator: create, generate_u256, or update
	RequestParentVersion     string // Authenticated immutable version fence observed before this write
	RequestExpectedUpdatedAt string // Authenticated optimistic timestamp fence supplied for an update
	CreatedAt                string //
}

// secretVersionColumns holds the columns for the table plugin_idc_core_secret_version.
var secretVersionColumns = SecretVersionColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	SecretNo:                 "secret_no",
	Version:                  "version",
	EnvelopeVersion:          "envelope_version",
	CipherAlgorithm:          "cipher_algorithm",
	PayloadNonce:             "payload_nonce",
	Ciphertext:               "ciphertext",
	RequestFingerprint:       "request_fingerprint",
	IdempotencyKeyDigest:     "idempotency_key_digest",
	RequestOperation:         "request_operation",
	RequestParentVersion:     "request_parent_version",
	RequestExpectedUpdatedAt: "request_expected_updated_at",
	CreatedAt:                "created_at",
}

// NewSecretVersionDao creates and returns a new DAO object for table data access.
func NewSecretVersionDao(handlers ...gdb.ModelHandler) *SecretVersionDao {
	return &SecretVersionDao{
		group:    "default",
		table:    "plugin_idc_core_secret_version",
		columns:  secretVersionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SecretVersionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SecretVersionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SecretVersionDao) Columns() SecretVersionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SecretVersionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SecretVersionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SecretVersionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
