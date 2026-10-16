// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesPaymentFactDao is the data access object for the table plugin_idc_core_sales_payment_fact.
type SalesPaymentFactDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  SalesPaymentFactColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// SalesPaymentFactColumns defines and stores column names for the table plugin_idc_core_sales_payment_fact.
type SalesPaymentFactColumns struct {
	Id                  string //
	TenantId            string //
	InvoiceNo           string //
	OrderNo             string //
	CustomerNo          string //
	CurrencyCode        string //
	OperationTypeCode   string //
	ReceivedAmountMinor string //
	ReceivedAt          string //
	SettlementNo        string //
	SourceDigest        string //
	CreatedAt           string //
}

// salesPaymentFactColumns holds the columns for the table plugin_idc_core_sales_payment_fact.
var salesPaymentFactColumns = SalesPaymentFactColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	InvoiceNo:           "invoice_no",
	OrderNo:             "order_no",
	CustomerNo:          "customer_no",
	CurrencyCode:        "currency_code",
	OperationTypeCode:   "operation_type_code",
	ReceivedAmountMinor: "received_amount_minor",
	ReceivedAt:          "received_at",
	SettlementNo:        "settlement_no",
	SourceDigest:        "source_digest",
	CreatedAt:           "created_at",
}

// NewSalesPaymentFactDao creates and returns a new DAO object for table data access.
func NewSalesPaymentFactDao(handlers ...gdb.ModelHandler) *SalesPaymentFactDao {
	return &SalesPaymentFactDao{
		group:    "default",
		table:    "plugin_idc_core_sales_payment_fact",
		columns:  salesPaymentFactColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesPaymentFactDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesPaymentFactDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesPaymentFactDao) Columns() SalesPaymentFactColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesPaymentFactDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesPaymentFactDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesPaymentFactDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
