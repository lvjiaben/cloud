// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// EnterpriseMaterialDao is the data access object for the table plugin_idc_compliance_enterprise_material.
type EnterpriseMaterialDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  EnterpriseMaterialColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// EnterpriseMaterialColumns defines and stores column names for the table plugin_idc_compliance_enterprise_material.
type EnterpriseMaterialColumns struct {
	Id                     string //
	TenantId               string //
	MaterialNo             string //
	CaseNo                 string //
	CaseSubjectTypeCode    string //
	MaterialTypeCode       string //
	SequenceNo             string //
	PrivateObjectRef       string //
	ContentSha256          string // SHA-256 of the encrypted storage envelope, never a digest derived directly from document plaintext
	MediaType              string //
	ByteSize               string //
	EncryptionStateCode    string //
	StatusCode             string //
	RetentionUntil         string //
	LegalHold              string //
	SupersededByMaterialNo string //
	CreatedAt              string //
	UpdatedAt              string //
	EncryptionKeyNo        string // Low-entropy keyring generation code; the secretcap handle remains in the keyring table only
	StoredByteSize         string // Encrypted envelope byte size confirmed against private object storage metadata
	IdempotencyKeyDigest   string //
	RequestFingerprint     string //
}

// enterpriseMaterialColumns holds the columns for the table plugin_idc_compliance_enterprise_material.
var enterpriseMaterialColumns = EnterpriseMaterialColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	MaterialNo:             "material_no",
	CaseNo:                 "case_no",
	CaseSubjectTypeCode:    "case_subject_type_code",
	MaterialTypeCode:       "material_type_code",
	SequenceNo:             "sequence_no",
	PrivateObjectRef:       "private_object_ref",
	ContentSha256:          "content_sha256",
	MediaType:              "media_type",
	ByteSize:               "byte_size",
	EncryptionStateCode:    "encryption_state_code",
	StatusCode:             "status_code",
	RetentionUntil:         "retention_until",
	LegalHold:              "legal_hold",
	SupersededByMaterialNo: "superseded_by_material_no",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
	EncryptionKeyNo:        "encryption_key_no",
	StoredByteSize:         "stored_byte_size",
	IdempotencyKeyDigest:   "idempotency_key_digest",
	RequestFingerprint:     "request_fingerprint",
}

// NewEnterpriseMaterialDao creates and returns a new DAO object for table data access.
func NewEnterpriseMaterialDao(handlers ...gdb.ModelHandler) *EnterpriseMaterialDao {
	return &EnterpriseMaterialDao{
		group:    "default",
		table:    "plugin_idc_compliance_enterprise_material",
		columns:  enterpriseMaterialColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *EnterpriseMaterialDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *EnterpriseMaterialDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *EnterpriseMaterialDao) Columns() EnterpriseMaterialColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *EnterpriseMaterialDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *EnterpriseMaterialDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *EnterpriseMaterialDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
