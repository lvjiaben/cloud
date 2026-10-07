// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyCustomerRequestDao is the data access object for the table plugin_idc_compliance_privacy_customer_request.
type PrivacyCustomerRequestDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  PrivacyCustomerRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// PrivacyCustomerRequestColumns defines and stores column names for the table plugin_idc_compliance_privacy_customer_request.
type PrivacyCustomerRequestColumns struct {
	Id              string //
	TenantId        string //
	RequestNo       string //
	CustomerNo      string //
	RequestTypeCode string //
	StatusCode      string //
	HoldReasonCode  string //
	SubmittedAt     string //
	CompletedAt     string //
	Version         string //
	CreatedAt       string //
	UpdatedAt       string //
}

// privacyCustomerRequestColumns holds the columns for the table plugin_idc_compliance_privacy_customer_request.
var privacyCustomerRequestColumns = PrivacyCustomerRequestColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	RequestNo:       "request_no",
	CustomerNo:      "customer_no",
	RequestTypeCode: "request_type_code",
	StatusCode:      "status_code",
	HoldReasonCode:  "hold_reason_code",
	SubmittedAt:     "submitted_at",
	CompletedAt:     "completed_at",
	Version:         "version",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewPrivacyCustomerRequestDao creates and returns a new DAO object for table data access.
func NewPrivacyCustomerRequestDao(handlers ...gdb.ModelHandler) *PrivacyCustomerRequestDao {
	return &PrivacyCustomerRequestDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_customer_request",
		columns:  privacyCustomerRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyCustomerRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyCustomerRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyCustomerRequestDao) Columns() PrivacyCustomerRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyCustomerRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyCustomerRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacyCustomerRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
