// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerSshPublicKeyDao is the data access object for the table plugin_idc_core_customer_ssh_public_key.
type CustomerSshPublicKeyDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  CustomerSshPublicKeyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// CustomerSshPublicKeyColumns defines and stores column names for the table plugin_idc_core_customer_ssh_public_key.
type CustomerSshPublicKeyColumns struct {
	Id                   string //
	TenantId             string //
	KeyNo                string //
	CustomerNo           string //
	Name                 string //
	AlgorithmCode        string //
	PublicKey            string //
	FingerprintSha256    string //
	RequestFingerprint   string //
	IdempotencyKeyDigest string //
	Version              string //
	CreatedAt            string //
	RevokedAt            string //
}

// customerSshPublicKeyColumns holds the columns for the table plugin_idc_core_customer_ssh_public_key.
var customerSshPublicKeyColumns = CustomerSshPublicKeyColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	KeyNo:                "key_no",
	CustomerNo:           "customer_no",
	Name:                 "name",
	AlgorithmCode:        "algorithm_code",
	PublicKey:            "public_key",
	FingerprintSha256:    "fingerprint_sha256",
	RequestFingerprint:   "request_fingerprint",
	IdempotencyKeyDigest: "idempotency_key_digest",
	Version:              "version",
	CreatedAt:            "created_at",
	RevokedAt:            "revoked_at",
}

// NewCustomerSshPublicKeyDao creates and returns a new DAO object for table data access.
func NewCustomerSshPublicKeyDao(handlers ...gdb.ModelHandler) *CustomerSshPublicKeyDao {
	return &CustomerSshPublicKeyDao{
		group:    "default",
		table:    "plugin_idc_core_customer_ssh_public_key",
		columns:  customerSshPublicKeyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerSshPublicKeyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerSshPublicKeyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerSshPublicKeyDao) Columns() CustomerSshPublicKeyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerSshPublicKeyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerSshPublicKeyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerSshPublicKeyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
