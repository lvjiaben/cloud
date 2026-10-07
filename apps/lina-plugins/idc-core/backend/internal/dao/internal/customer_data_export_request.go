// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerDataExportRequestDao is the data access object for the table plugin_idc_core_customer_data_export_request.
type CustomerDataExportRequestDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CustomerDataExportRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CustomerDataExportRequestColumns defines and stores column names for the table plugin_idc_core_customer_data_export_request.
type CustomerDataExportRequestColumns struct {
	Id                   string //
	TenantId             string //
	RequestNo            string //
	CustomerNo           string //
	ConsentNo            string //
	TaskNo               string // Application-validated asynchronous task reference without a cross-domain database foreign key
	ScopeCode            string //
	FormatCode           string //
	SubjectVersion       string //
	IdempotencyKeyDigest string //
	RequestFingerprint   string //
	ReauthIssuedAt       string //
	RequestedByUserId    string //
	RequestedAt          string //
	CreatedAt            string //
	UpdatedAt            string //
}

// customerDataExportRequestColumns holds the columns for the table plugin_idc_core_customer_data_export_request.
var customerDataExportRequestColumns = CustomerDataExportRequestColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	RequestNo:            "request_no",
	CustomerNo:           "customer_no",
	ConsentNo:            "consent_no",
	TaskNo:               "task_no",
	ScopeCode:            "scope_code",
	FormatCode:           "format_code",
	SubjectVersion:       "subject_version",
	IdempotencyKeyDigest: "idempotency_key_digest",
	RequestFingerprint:   "request_fingerprint",
	ReauthIssuedAt:       "reauth_issued_at",
	RequestedByUserId:    "requested_by_user_id",
	RequestedAt:          "requested_at",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewCustomerDataExportRequestDao creates and returns a new DAO object for table data access.
func NewCustomerDataExportRequestDao(handlers ...gdb.ModelHandler) *CustomerDataExportRequestDao {
	return &CustomerDataExportRequestDao{
		group:    "default",
		table:    "plugin_idc_core_customer_data_export_request",
		columns:  customerDataExportRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerDataExportRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerDataExportRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerDataExportRequestDao) Columns() CustomerDataExportRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerDataExportRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerDataExportRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerDataExportRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
