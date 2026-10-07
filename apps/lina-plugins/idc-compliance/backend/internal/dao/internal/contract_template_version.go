// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateVersionDao is the data access object for the table plugin_idc_compliance_contract_template_version.
type ContractTemplateVersionDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  ContractTemplateVersionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// ContractTemplateVersionColumns defines and stores column names for the table plugin_idc_compliance_contract_template_version.
type ContractTemplateVersionColumns struct {
	Id                             string //
	TenantId                       string //
	TemplateNo                     string //
	VersionNo                      string //
	Name                           string //
	ScopeCode                      string //
	SigningOrderCode               string //
	LanguageCode                   string // Explicit contract language; product and service names must use the matching locale
	StatusCode                     string //
	ProviderVersion                string //
	ProviderSnapshotDigest         string //
	ProviderCustomerSignatureCount string //
	ProviderCustomerSealCount      string //
	ProviderMerchantSignatureCount string //
	ProviderMerchantSealCount      string //
	CurrentContentRevisionNo       string // Opaque pointer to the latest append-only encrypted source and PDF revision
	CurrentFieldSetNo              string //
	PreviewConfirmed               string //
	UsedCount                      string //
	LockVersion                    string //
	CreatedAt                      string //
	UpdatedAt                      string //
	CurrentProviderMappingSetNo    string //
}

// contractTemplateVersionColumns holds the columns for the table plugin_idc_compliance_contract_template_version.
var contractTemplateVersionColumns = ContractTemplateVersionColumns{
	Id:                             "id",
	TenantId:                       "tenant_id",
	TemplateNo:                     "template_no",
	VersionNo:                      "version_no",
	Name:                           "name",
	ScopeCode:                      "scope_code",
	SigningOrderCode:               "signing_order_code",
	LanguageCode:                   "language_code",
	StatusCode:                     "status_code",
	ProviderVersion:                "provider_version",
	ProviderSnapshotDigest:         "provider_snapshot_digest",
	ProviderCustomerSignatureCount: "provider_customer_signature_count",
	ProviderCustomerSealCount:      "provider_customer_seal_count",
	ProviderMerchantSignatureCount: "provider_merchant_signature_count",
	ProviderMerchantSealCount:      "provider_merchant_seal_count",
	CurrentContentRevisionNo:       "current_content_revision_no",
	CurrentFieldSetNo:              "current_field_set_no",
	PreviewConfirmed:               "preview_confirmed",
	UsedCount:                      "used_count",
	LockVersion:                    "lock_version",
	CreatedAt:                      "created_at",
	UpdatedAt:                      "updated_at",
	CurrentProviderMappingSetNo:    "current_provider_mapping_set_no",
}

// NewContractTemplateVersionDao creates and returns a new DAO object for table data access.
func NewContractTemplateVersionDao(handlers ...gdb.ModelHandler) *ContractTemplateVersionDao {
	return &ContractTemplateVersionDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_template_version",
		columns:  contractTemplateVersionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractTemplateVersionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractTemplateVersionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractTemplateVersionDao) Columns() ContractTemplateVersionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractTemplateVersionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractTemplateVersionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractTemplateVersionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
