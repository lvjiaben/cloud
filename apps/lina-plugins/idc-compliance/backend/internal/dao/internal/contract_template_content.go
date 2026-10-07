// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateContentDao is the data access object for the table plugin_idc_compliance_contract_template_content.
type ContractTemplateContentDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  ContractTemplateContentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// ContractTemplateContentColumns defines and stores column names for the table plugin_idc_compliance_contract_template_content.
type ContractTemplateContentColumns struct {
	Id                       string //
	TenantId                 string //
	ContentRevisionNo        string //
	TemplateNo               string //
	VersionNo                string //
	RevisionNo               string //
	CommandIdempotencyDigest string //
	CommandFingerprint       string //
	SourceModeCode           string //
	EncryptionKeyNo          string //
	SourceObjectRef          string //
	SourceSha256             string //
	SourceMediaType          string //
	SourceByteSize           string //
	PdfObjectRef             string //
	PdfSha256                string //
	PdfByteSize              string //
	PdfPageCount             string //
	ConversionStatusCode     string //
	ProviderFileRef          string //
	CreatedByUserId          string //
	CreatedAt                string //
}

// contractTemplateContentColumns holds the columns for the table plugin_idc_compliance_contract_template_content.
var contractTemplateContentColumns = ContractTemplateContentColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	ContentRevisionNo:        "content_revision_no",
	TemplateNo:               "template_no",
	VersionNo:                "version_no",
	RevisionNo:               "revision_no",
	CommandIdempotencyDigest: "command_idempotency_digest",
	CommandFingerprint:       "command_fingerprint",
	SourceModeCode:           "source_mode_code",
	EncryptionKeyNo:          "encryption_key_no",
	SourceObjectRef:          "source_object_ref",
	SourceSha256:             "source_sha256",
	SourceMediaType:          "source_media_type",
	SourceByteSize:           "source_byte_size",
	PdfObjectRef:             "pdf_object_ref",
	PdfSha256:                "pdf_sha256",
	PdfByteSize:              "pdf_byte_size",
	PdfPageCount:             "pdf_page_count",
	ConversionStatusCode:     "conversion_status_code",
	ProviderFileRef:          "provider_file_ref",
	CreatedByUserId:          "created_by_user_id",
	CreatedAt:                "created_at",
}

// NewContractTemplateContentDao creates and returns a new DAO object for table data access.
func NewContractTemplateContentDao(handlers ...gdb.ModelHandler) *ContractTemplateContentDao {
	return &ContractTemplateContentDao{
		group:    "default",
		table:    "plugin_idc_compliance_contract_template_content",
		columns:  contractTemplateContentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContractTemplateContentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContractTemplateContentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContractTemplateContentDao) Columns() ContractTemplateContentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContractTemplateContentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContractTemplateContentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContractTemplateContentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
