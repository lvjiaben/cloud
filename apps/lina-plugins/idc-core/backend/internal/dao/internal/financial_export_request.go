// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialExportRequestDao is the data access object for the table plugin_idc_core_financial_export_request.
type FinancialExportRequestDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  FinancialExportRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// FinancialExportRequestColumns defines and stores column names for the table plugin_idc_core_financial_export_request.
type FinancialExportRequestColumns struct {
	Id                     string //
	TenantId               string //
	RequestNo              string //
	ExportTypeCode         string //
	FilterJson             string //
	FilterDigest           string //
	PermissionCode         string //
	DataScopeCode          string //
	RequestedBy            string //
	IdempotencyDigest      string //
	AuthorizationCheckedAt string //
	StatusCode             string //
	FailureCode            string //
	GeneratedAt            string //
	CreatedAt              string //
}

// financialExportRequestColumns holds the columns for the table plugin_idc_core_financial_export_request.
var financialExportRequestColumns = FinancialExportRequestColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	RequestNo:              "request_no",
	ExportTypeCode:         "export_type_code",
	FilterJson:             "filter_json",
	FilterDigest:           "filter_digest",
	PermissionCode:         "permission_code",
	DataScopeCode:          "data_scope_code",
	RequestedBy:            "requested_by",
	IdempotencyDigest:      "idempotency_digest",
	AuthorizationCheckedAt: "authorization_checked_at",
	StatusCode:             "status_code",
	FailureCode:            "failure_code",
	GeneratedAt:            "generated_at",
	CreatedAt:              "created_at",
}

// NewFinancialExportRequestDao creates and returns a new DAO object for table data access.
func NewFinancialExportRequestDao(handlers ...gdb.ModelHandler) *FinancialExportRequestDao {
	return &FinancialExportRequestDao{
		group:    "default",
		table:    "plugin_idc_core_financial_export_request",
		columns:  financialExportRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialExportRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialExportRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialExportRequestDao) Columns() FinancialExportRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialExportRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialExportRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialExportRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
