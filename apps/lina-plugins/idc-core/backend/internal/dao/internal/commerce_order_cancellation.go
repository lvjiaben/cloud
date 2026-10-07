// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderCancellationDao is the data access object for the table plugin_idc_core_commerce_order_cancellation.
type CommerceOrderCancellationDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CommerceOrderCancellationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CommerceOrderCancellationColumns defines and stores column names for the table plugin_idc_core_commerce_order_cancellation.
type CommerceOrderCancellationColumns struct {
	Id                        string //
	TenantId                  string //
	CancellationNo            string //
	OrderNo                   string //
	CustomerNo                string //
	SourceCode                string //
	ReasonCode                string //
	RequestDigest             string //
	StatusCode                string //
	PaymentClosureOutcomeCode string //
	PaymentClosureReference   string //
	PaymentClosureDigest      string //
	RequestedBy               string //
	RequestedAt               string //
	CompletedAt               string //
	Version                   string //
	CreatedAt                 string //
	UpdatedAt                 string //
}

// commerceOrderCancellationColumns holds the columns for the table plugin_idc_core_commerce_order_cancellation.
var commerceOrderCancellationColumns = CommerceOrderCancellationColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	CancellationNo:            "cancellation_no",
	OrderNo:                   "order_no",
	CustomerNo:                "customer_no",
	SourceCode:                "source_code",
	ReasonCode:                "reason_code",
	RequestDigest:             "request_digest",
	StatusCode:                "status_code",
	PaymentClosureOutcomeCode: "payment_closure_outcome_code",
	PaymentClosureReference:   "payment_closure_reference",
	PaymentClosureDigest:      "payment_closure_digest",
	RequestedBy:               "requested_by",
	RequestedAt:               "requested_at",
	CompletedAt:               "completed_at",
	Version:                   "version",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
}

// NewCommerceOrderCancellationDao creates and returns a new DAO object for table data access.
func NewCommerceOrderCancellationDao(handlers ...gdb.ModelHandler) *CommerceOrderCancellationDao {
	return &CommerceOrderCancellationDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_order_cancellation",
		columns:  commerceOrderCancellationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceOrderCancellationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceOrderCancellationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceOrderCancellationDao) Columns() CommerceOrderCancellationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceOrderCancellationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceOrderCancellationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceOrderCancellationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
