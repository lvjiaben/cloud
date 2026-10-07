// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupplierConfigurationDao is the data access object for the table plugin_idc_compliance_supplier_configuration.
type SupplierConfigurationDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SupplierConfigurationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SupplierConfigurationColumns defines and stores column names for the table plugin_idc_compliance_supplier_configuration.
type SupplierConfigurationColumns struct {
	Id                      string //
	TenantId                string //
	ConfigurationNo         string //
	ProviderCode            string //
	CredentialStatusCode    string // Credential lifecycle only; independent from supplier subject binding state
	CredentialSecretRef     string // Sensitive secretcap routing reference for one provider-neutral credential envelope; plaintext and ciphertext are forbidden
	CredentialGeneration    string //
	CapabilityEvidenceCode  string // Reviewed provider-contract gate; official_public_contract is valid only for Tencent ESS Basic
	AdapterEnabled          string // True only for an active Tencent local subject backed by the reviewed public API contract
	LastMutationDigest      string //
	MutationOperationKey    string //
	MutationSecretUpdatedAt string //
	Version                 string //
	CreatedAt               string //
	UpdatedAt               string //
	SubjectStatusCode       string //
	SubjectBindingNo        string // Opaque installation-local gateway binding identifier; supplier subject plaintext is forbidden
	LastOperationKindCode   string //
	LastOperationResultCode string //
	LastOperationNo         string // Opaque plugin-local operation identifier; provider payloads and subject identifiers are forbidden
	LastOperationAt         string //
	SubjectProofDigest      string // Digest of the current server-only single-use proof; raw proof tokens are forbidden
	SubjectProofExpiresAt   string //
}

// supplierConfigurationColumns holds the columns for the table plugin_idc_compliance_supplier_configuration.
var supplierConfigurationColumns = SupplierConfigurationColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	ConfigurationNo:         "configuration_no",
	ProviderCode:            "provider_code",
	CredentialStatusCode:    "credential_status_code",
	CredentialSecretRef:     "credential_secret_ref",
	CredentialGeneration:    "credential_generation",
	CapabilityEvidenceCode:  "capability_evidence_code",
	AdapterEnabled:          "adapter_enabled",
	LastMutationDigest:      "last_mutation_digest",
	MutationOperationKey:    "mutation_operation_key",
	MutationSecretUpdatedAt: "mutation_secret_updated_at",
	Version:                 "version",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
	SubjectStatusCode:       "subject_status_code",
	SubjectBindingNo:        "subject_binding_no",
	LastOperationKindCode:   "last_operation_kind_code",
	LastOperationResultCode: "last_operation_result_code",
	LastOperationNo:         "last_operation_no",
	LastOperationAt:         "last_operation_at",
	SubjectProofDigest:      "subject_proof_digest",
	SubjectProofExpiresAt:   "subject_proof_expires_at",
}

// NewSupplierConfigurationDao creates and returns a new DAO object for table data access.
func NewSupplierConfigurationDao(handlers ...gdb.ModelHandler) *SupplierConfigurationDao {
	return &SupplierConfigurationDao{
		group:    "default",
		table:    "plugin_idc_compliance_supplier_configuration",
		columns:  supplierConfigurationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupplierConfigurationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupplierConfigurationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupplierConfigurationDao) Columns() SupplierConfigurationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupplierConfigurationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupplierConfigurationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupplierConfigurationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
