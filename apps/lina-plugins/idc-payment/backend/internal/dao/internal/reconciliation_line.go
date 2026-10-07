// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationLineDao is the data access object for the table plugin_idc_payment_reconciliation_line.
type ReconciliationLineDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  ReconciliationLineColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// ReconciliationLineColumns defines and stores column names for the table plugin_idc_payment_reconciliation_line.
type ReconciliationLineColumns struct {
	Id                 string //
	LineNo             string //
	BatchNo            string //
	AttemptNo          string //
	RefundAttemptNo    string //
	ExternalReference  string //
	DifferenceCode     string //
	LocalAmountMinor   string //
	ChannelAmountMinor string //
	CurrencyCode       string //
	ResolutionCode     string //
	EvidenceDigest     string //
	CreatedAt          string //
	ResolvedAt         string //
	RecordTypeCode     string //
	CustomerRefDigest  string //
	LocalStatusCode    string //
	ChannelStatusCode  string //
	Version            string //
}

// reconciliationLineColumns holds the columns for the table plugin_idc_payment_reconciliation_line.
var reconciliationLineColumns = ReconciliationLineColumns{
	Id:                 "id",
	LineNo:             "line_no",
	BatchNo:            "batch_no",
	AttemptNo:          "attempt_no",
	RefundAttemptNo:    "refund_attempt_no",
	ExternalReference:  "external_reference",
	DifferenceCode:     "difference_code",
	LocalAmountMinor:   "local_amount_minor",
	ChannelAmountMinor: "channel_amount_minor",
	CurrencyCode:       "currency_code",
	ResolutionCode:     "resolution_code",
	EvidenceDigest:     "evidence_digest",
	CreatedAt:          "created_at",
	ResolvedAt:         "resolved_at",
	RecordTypeCode:     "record_type_code",
	CustomerRefDigest:  "customer_ref_digest",
	LocalStatusCode:    "local_status_code",
	ChannelStatusCode:  "channel_status_code",
	Version:            "version",
}

// NewReconciliationLineDao creates and returns a new DAO object for table data access.
func NewReconciliationLineDao(handlers ...gdb.ModelHandler) *ReconciliationLineDao {
	return &ReconciliationLineDao{
		group:    "default",
		table:    "plugin_idc_payment_reconciliation_line",
		columns:  reconciliationLineColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ReconciliationLineDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ReconciliationLineDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ReconciliationLineDao) Columns() ReconciliationLineColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ReconciliationLineDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ReconciliationLineDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ReconciliationLineDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
