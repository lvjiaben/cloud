// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialDeliveryIntentDao is the data access object for the table plugin_idc_core_financial_delivery_intent.
type FinancialDeliveryIntentDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  FinancialDeliveryIntentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// FinancialDeliveryIntentColumns defines and stores column names for the table plugin_idc_core_financial_delivery_intent.
type FinancialDeliveryIntentColumns struct {
	Id                  string //
	TenantId            string //
	IntentNo            string //
	InvoiceNo           string //
	CustomerNo          string //
	ContactNo           string //
	InvoiceVersion      string //
	NotificationEventNo string //
	RequestedBy         string //
	IdempotencyDigest   string //
	RequestDigest       string //
	CreatedAt           string //
}

// financialDeliveryIntentColumns holds the columns for the table plugin_idc_core_financial_delivery_intent.
var financialDeliveryIntentColumns = FinancialDeliveryIntentColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	IntentNo:            "intent_no",
	InvoiceNo:           "invoice_no",
	CustomerNo:          "customer_no",
	ContactNo:           "contact_no",
	InvoiceVersion:      "invoice_version",
	NotificationEventNo: "notification_event_no",
	RequestedBy:         "requested_by",
	IdempotencyDigest:   "idempotency_digest",
	RequestDigest:       "request_digest",
	CreatedAt:           "created_at",
}

// NewFinancialDeliveryIntentDao creates and returns a new DAO object for table data access.
func NewFinancialDeliveryIntentDao(handlers ...gdb.ModelHandler) *FinancialDeliveryIntentDao {
	return &FinancialDeliveryIntentDao{
		group:    "default",
		table:    "plugin_idc_core_financial_delivery_intent",
		columns:  financialDeliveryIntentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialDeliveryIntentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialDeliveryIntentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialDeliveryIntentDao) Columns() FinancialDeliveryIntentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialDeliveryIntentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialDeliveryIntentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialDeliveryIntentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
