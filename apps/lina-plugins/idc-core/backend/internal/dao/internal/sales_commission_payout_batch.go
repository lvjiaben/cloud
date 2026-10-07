// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutBatchDao is the data access object for the table plugin_idc_core_sales_commission_payout_batch.
type SalesCommissionPayoutBatchDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  SalesCommissionPayoutBatchColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// SalesCommissionPayoutBatchColumns defines and stores column names for the table plugin_idc_core_sales_commission_payout_batch.
type SalesCommissionPayoutBatchColumns struct {
	Id                   string //
	TenantId             string //
	BatchNo              string //
	CurrencyCode         string //
	ItemCount            string //
	TotalAmountMinor     string //
	SucceededCount       string //
	SucceededAmountMinor string //
	FailedCount          string //
	FailedAmountMinor    string //
	StatusCode           string //
	CreatedBy            string //
	CreatedAt            string //
	CompletedAt          string //
}

// salesCommissionPayoutBatchColumns holds the columns for the table plugin_idc_core_sales_commission_payout_batch.
var salesCommissionPayoutBatchColumns = SalesCommissionPayoutBatchColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	BatchNo:              "batch_no",
	CurrencyCode:         "currency_code",
	ItemCount:            "item_count",
	TotalAmountMinor:     "total_amount_minor",
	SucceededCount:       "succeeded_count",
	SucceededAmountMinor: "succeeded_amount_minor",
	FailedCount:          "failed_count",
	FailedAmountMinor:    "failed_amount_minor",
	StatusCode:           "status_code",
	CreatedBy:            "created_by",
	CreatedAt:            "created_at",
	CompletedAt:          "completed_at",
}

// NewSalesCommissionPayoutBatchDao creates and returns a new DAO object for table data access.
func NewSalesCommissionPayoutBatchDao(handlers ...gdb.ModelHandler) *SalesCommissionPayoutBatchDao {
	return &SalesCommissionPayoutBatchDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_payout_batch",
		columns:  salesCommissionPayoutBatchColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionPayoutBatchDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionPayoutBatchDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionPayoutBatchDao) Columns() SalesCommissionPayoutBatchColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionPayoutBatchDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionPayoutBatchDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionPayoutBatchDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
