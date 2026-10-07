// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProviderConfigurationDao is the data access object for the table plugin_idc_compliance_provider_configuration.
type ProviderConfigurationDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  ProviderConfigurationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// ProviderConfigurationColumns defines and stores column names for the table plugin_idc_compliance_provider_configuration.
type ProviderConfigurationColumns struct {
	Id                      string //
	TenantId                string //
	ConfigurationNo         string //
	ProviderCode            string //
	StatusCode              string //
	SceneId                 string // Merchant-owned Aliyun financial-grade identity verification scene ID; endpoint, service, version, FACE code, and identity type are fixed in code
	CredentialSecretRef     string // Sensitive secretcap routing reference for one atomic RAM AccessKey credential envelope; plaintext and ciphertext are forbidden
	CredentialGeneration    string //
	LastMutationDigest      string // SHA-256 digest of the bounded operator idempotency key; credentials are excluded
	MutationOperationKey    string // Internal random secretcap idempotency key retained only for audited recovery; never exposed by API, logs, or audit payloads
	MutationSecretUpdatedAt string // Safe secretcap optimistic fence retained to resume or diagnose the last idempotent credential replacement
	LastValidationCode      string //
	ValidatedAt             string //
	Version                 string //
	CreatedAt               string //
	UpdatedAt               string //
}

// providerConfigurationColumns holds the columns for the table plugin_idc_compliance_provider_configuration.
var providerConfigurationColumns = ProviderConfigurationColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	ConfigurationNo:         "configuration_no",
	ProviderCode:            "provider_code",
	StatusCode:              "status_code",
	SceneId:                 "scene_id",
	CredentialSecretRef:     "credential_secret_ref",
	CredentialGeneration:    "credential_generation",
	LastMutationDigest:      "last_mutation_digest",
	MutationOperationKey:    "mutation_operation_key",
	MutationSecretUpdatedAt: "mutation_secret_updated_at",
	LastValidationCode:      "last_validation_code",
	ValidatedAt:             "validated_at",
	Version:                 "version",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewProviderConfigurationDao creates and returns a new DAO object for table data access.
func NewProviderConfigurationDao(handlers ...gdb.ModelHandler) *ProviderConfigurationDao {
	return &ProviderConfigurationDao{
		group:    "default",
		table:    "plugin_idc_compliance_provider_configuration",
		columns:  providerConfigurationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProviderConfigurationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProviderConfigurationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProviderConfigurationDao) Columns() ProviderConfigurationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProviderConfigurationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProviderConfigurationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProviderConfigurationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
