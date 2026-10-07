// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialExportArtifactDao is the data access object for the table plugin_idc_core_financial_export_artifact.
type FinancialExportArtifactDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  FinancialExportArtifactColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// FinancialExportArtifactColumns defines and stores column names for the table plugin_idc_core_financial_export_artifact.
type FinancialExportArtifactColumns struct {
	Id            string //
	TenantId      string //
	RequestNo     string //
	ObjectRef     string //
	ContentSha256 string //
	SizeBytes     string //
	RowCount      string //
	StatusCode    string //
	ExpiresAt     string //
	CreatedAt     string //
	UpdatedAt     string //
}

// financialExportArtifactColumns holds the columns for the table plugin_idc_core_financial_export_artifact.
var financialExportArtifactColumns = FinancialExportArtifactColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	RequestNo:     "request_no",
	ObjectRef:     "object_ref",
	ContentSha256: "content_sha256",
	SizeBytes:     "size_bytes",
	RowCount:      "row_count",
	StatusCode:    "status_code",
	ExpiresAt:     "expires_at",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewFinancialExportArtifactDao creates and returns a new DAO object for table data access.
func NewFinancialExportArtifactDao(handlers ...gdb.ModelHandler) *FinancialExportArtifactDao {
	return &FinancialExportArtifactDao{
		group:    "default",
		table:    "plugin_idc_core_financial_export_artifact",
		columns:  financialExportArtifactColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialExportArtifactDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialExportArtifactDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialExportArtifactDao) Columns() FinancialExportArtifactColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialExportArtifactDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialExportArtifactDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialExportArtifactDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
