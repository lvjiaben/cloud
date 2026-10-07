// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacySupportExportDao is the data access object for the table plugin_idc_compliance_privacy_support_export.
type PrivacySupportExportDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  PrivacySupportExportColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// PrivacySupportExportColumns defines and stores column names for the table plugin_idc_compliance_privacy_support_export.
type PrivacySupportExportColumns struct {
	Id              string //
	TenantId        string //
	ExportNo        string //
	PurposeCode     string //
	AssessmentNo    string //
	FieldCount      string //
	RecordCount     string //
	ContentSha256   string //
	ContentBytes    string //
	StatusCode      string //
	RetentionUntil  string //
	CreatedByUserId string //
	CreatedAt       string //
}

// privacySupportExportColumns holds the columns for the table plugin_idc_compliance_privacy_support_export.
var privacySupportExportColumns = PrivacySupportExportColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	ExportNo:        "export_no",
	PurposeCode:     "purpose_code",
	AssessmentNo:    "assessment_no",
	FieldCount:      "field_count",
	RecordCount:     "record_count",
	ContentSha256:   "content_sha256",
	ContentBytes:    "content_bytes",
	StatusCode:      "status_code",
	RetentionUntil:  "retention_until",
	CreatedByUserId: "created_by_user_id",
	CreatedAt:       "created_at",
}

// NewPrivacySupportExportDao creates and returns a new DAO object for table data access.
func NewPrivacySupportExportDao(handlers ...gdb.ModelHandler) *PrivacySupportExportDao {
	return &PrivacySupportExportDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_support_export",
		columns:  privacySupportExportColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacySupportExportDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacySupportExportDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacySupportExportDao) Columns() PrivacySupportExportColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacySupportExportDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacySupportExportDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacySupportExportDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
