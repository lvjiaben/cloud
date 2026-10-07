// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationBatchDao is the data access object for the table plugin_idc_payment_reconciliation_batch.
type ReconciliationBatchDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  ReconciliationBatchColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// ReconciliationBatchColumns defines and stores column names for the table plugin_idc_payment_reconciliation_batch.
type ReconciliationBatchColumns struct {
	Id               string //
	BatchNo          string //
	ChannelNo        string //
	BusinessDate     string //
	SourceDigest     string //
	StatusCode       string //
	TotalCount       string //
	DifferenceCount  string //
	CreatedAt        string //
	FinishedAt       string //
	DailyScopeKey    string //
	MerchantTimezone string //
	SourceTypeCode   string //
	CapabilityCode   string //
	PaymentCount     string //
	RefundCount      string //
	ResultUnknown    string //
	FailureCode      string //
	UpdatedAt        string //
	Version          string //
}

// reconciliationBatchColumns holds the columns for the table plugin_idc_payment_reconciliation_batch.
var reconciliationBatchColumns = ReconciliationBatchColumns{
	Id:               "id",
	BatchNo:          "batch_no",
	ChannelNo:        "channel_no",
	BusinessDate:     "business_date",
	SourceDigest:     "source_digest",
	StatusCode:       "status_code",
	TotalCount:       "total_count",
	DifferenceCount:  "difference_count",
	CreatedAt:        "created_at",
	FinishedAt:       "finished_at",
	DailyScopeKey:    "daily_scope_key",
	MerchantTimezone: "merchant_timezone",
	SourceTypeCode:   "source_type_code",
	CapabilityCode:   "capability_code",
	PaymentCount:     "payment_count",
	RefundCount:      "refund_count",
	ResultUnknown:    "result_unknown",
	FailureCode:      "failure_code",
	UpdatedAt:        "updated_at",
	Version:          "version",
}

// NewReconciliationBatchDao creates and returns a new DAO object for table data access.
func NewReconciliationBatchDao(handlers ...gdb.ModelHandler) *ReconciliationBatchDao {
	return &ReconciliationBatchDao{
		group:    "default",
		table:    "plugin_idc_payment_reconciliation_batch",
		columns:  reconciliationBatchColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ReconciliationBatchDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ReconciliationBatchDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ReconciliationBatchDao) Columns() ReconciliationBatchColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ReconciliationBatchDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ReconciliationBatchDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ReconciliationBatchDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
