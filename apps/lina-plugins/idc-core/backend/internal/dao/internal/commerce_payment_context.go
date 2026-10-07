// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommercePaymentContextDao is the data access object for the table plugin_idc_core_commerce_payment_context.
type CommercePaymentContextDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  CommercePaymentContextColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// CommercePaymentContextColumns defines and stores column names for the table plugin_idc_core_commerce_payment_context.
type CommercePaymentContextColumns struct {
	Id                 string //
	TenantId           string //
	ContextNo          string //
	OrderNo            string //
	InvoiceNo          string //
	CustomerNo         string //
	CustomerTypeCode   string //
	StatusCode         string //
	ReasonCode         string //
	IdentityStatusCode string //
	IdentityCaseNo     string // Opaque compliance-owned case reference from the most recent evaluation, never raw identity material
	IdentityVersion    string //
	FirstRequestedAt   string //
	LastEvaluatedAt    string //
	ReadyAt            string //
	ExpiresAt          string //
	Version            string //
	CreatedAt          string //
	UpdatedAt          string //
}

// commercePaymentContextColumns holds the columns for the table plugin_idc_core_commerce_payment_context.
var commercePaymentContextColumns = CommercePaymentContextColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	ContextNo:          "context_no",
	OrderNo:            "order_no",
	InvoiceNo:          "invoice_no",
	CustomerNo:         "customer_no",
	CustomerTypeCode:   "customer_type_code",
	StatusCode:         "status_code",
	ReasonCode:         "reason_code",
	IdentityStatusCode: "identity_status_code",
	IdentityCaseNo:     "identity_case_no",
	IdentityVersion:    "identity_version",
	FirstRequestedAt:   "first_requested_at",
	LastEvaluatedAt:    "last_evaluated_at",
	ReadyAt:            "ready_at",
	ExpiresAt:          "expires_at",
	Version:            "version",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
}

// NewCommercePaymentContextDao creates and returns a new DAO object for table data access.
func NewCommercePaymentContextDao(handlers ...gdb.ModelHandler) *CommercePaymentContextDao {
	return &CommercePaymentContextDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_payment_context",
		columns:  commercePaymentContextColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommercePaymentContextDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommercePaymentContextDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommercePaymentContextDao) Columns() CommercePaymentContextColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommercePaymentContextDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommercePaymentContextDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommercePaymentContextDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
