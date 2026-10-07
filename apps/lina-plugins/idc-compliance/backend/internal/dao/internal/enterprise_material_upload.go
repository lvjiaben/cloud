// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// EnterpriseMaterialUploadDao is the data access object for the table plugin_idc_compliance_enterprise_material_upload.
type EnterpriseMaterialUploadDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  EnterpriseMaterialUploadColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// EnterpriseMaterialUploadColumns defines and stores column names for the table plugin_idc_compliance_enterprise_material_upload.
type EnterpriseMaterialUploadColumns struct {
	Id                   string //
	TenantId             string //
	MaterialNo           string //
	CaseNo               string //
	CaseSubjectTypeCode  string //
	MaterialTypeCode     string //
	IdempotencyKeyDigest string //
	RequestFingerprint   string //
	StatusCode           string //
	Version              string //
	CreatedAt            string //
	UpdatedAt            string //
}

// enterpriseMaterialUploadColumns holds the columns for the table plugin_idc_compliance_enterprise_material_upload.
var enterpriseMaterialUploadColumns = EnterpriseMaterialUploadColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	MaterialNo:           "material_no",
	CaseNo:               "case_no",
	CaseSubjectTypeCode:  "case_subject_type_code",
	MaterialTypeCode:     "material_type_code",
	IdempotencyKeyDigest: "idempotency_key_digest",
	RequestFingerprint:   "request_fingerprint",
	StatusCode:           "status_code",
	Version:              "version",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewEnterpriseMaterialUploadDao creates and returns a new DAO object for table data access.
func NewEnterpriseMaterialUploadDao(handlers ...gdb.ModelHandler) *EnterpriseMaterialUploadDao {
	return &EnterpriseMaterialUploadDao{
		group:    "default",
		table:    "plugin_idc_compliance_enterprise_material_upload",
		columns:  enterpriseMaterialUploadColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *EnterpriseMaterialUploadDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *EnterpriseMaterialUploadDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *EnterpriseMaterialUploadDao) Columns() EnterpriseMaterialUploadColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *EnterpriseMaterialUploadDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *EnterpriseMaterialUploadDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *EnterpriseMaterialUploadDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
