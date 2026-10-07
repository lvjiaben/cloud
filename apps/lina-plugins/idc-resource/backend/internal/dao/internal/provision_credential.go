// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProvisionCredentialDao is the data access object for the table plugin_idc_resource_provision_credential.
type ProvisionCredentialDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  ProvisionCredentialColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// ProvisionCredentialColumns defines and stores column names for the table plugin_idc_resource_provision_credential.
type ProvisionCredentialColumns struct {
	Id           string //
	TenantId     string //
	CredentialNo string //
	ServiceNo    string //
	TaskNo       string //
	ChannelNo    string //
	SecretRef    string // Sensitive secretcap handle reference; never returned, logged, audited, or sent as ordinary JSON
	StatusCode   string //
	RevealedAt   string //
	RevokedAt    string //
	Version      string //
	CreatedAt    string //
	UpdatedAt    string //
}

// provisionCredentialColumns holds the columns for the table plugin_idc_resource_provision_credential.
var provisionCredentialColumns = ProvisionCredentialColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	CredentialNo: "credential_no",
	ServiceNo:    "service_no",
	TaskNo:       "task_no",
	ChannelNo:    "channel_no",
	SecretRef:    "secret_ref",
	StatusCode:   "status_code",
	RevealedAt:   "revealed_at",
	RevokedAt:    "revoked_at",
	Version:      "version",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewProvisionCredentialDao creates and returns a new DAO object for table data access.
func NewProvisionCredentialDao(handlers ...gdb.ModelHandler) *ProvisionCredentialDao {
	return &ProvisionCredentialDao{
		group:    "default",
		table:    "plugin_idc_resource_provision_credential",
		columns:  provisionCredentialColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProvisionCredentialDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProvisionCredentialDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProvisionCredentialDao) Columns() ProvisionCredentialColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProvisionCredentialDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProvisionCredentialDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProvisionCredentialDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
