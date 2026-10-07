// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AttemptDao is the data access object for the table plugin_idc_payment_attempt.
type AttemptDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  AttemptColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// AttemptColumns defines and stores column names for the table plugin_idc_payment_attempt.
type AttemptColumns struct {
	Id                     string //
	TenantId               string //
	AttemptNo              string //
	ChannelNo              string //
	InvoiceNo              string //
	OrderNo                string //
	PurposeCode            string //
	PurposeReference       string //
	CustomerRefDigest      string //
	CurrencyCode           string //
	AmountMinor            string //
	BalanceLockReference   string //
	BalanceLockedMinor     string //
	ModeCode               string //
	RequestDigest          string //
	ExternalTradeReference string //
	StatusCode             string //
	ExpiresAt              string //
	SubmittedAt            string //
	ConfirmedAt            string //
	ClosedAt               string //
	ResultUnknown          string //
	FundingResultDigest    string //
	Version                string //
	CreatedAt              string //
	UpdatedAt              string //
}

// attemptColumns holds the columns for the table plugin_idc_payment_attempt.
var attemptColumns = AttemptColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	AttemptNo:              "attempt_no",
	ChannelNo:              "channel_no",
	InvoiceNo:              "invoice_no",
	OrderNo:                "order_no",
	PurposeCode:            "purpose_code",
	PurposeReference:       "purpose_reference",
	CustomerRefDigest:      "customer_ref_digest",
	CurrencyCode:           "currency_code",
	AmountMinor:            "amount_minor",
	BalanceLockReference:   "balance_lock_reference",
	BalanceLockedMinor:     "balance_locked_minor",
	ModeCode:               "mode_code",
	RequestDigest:          "request_digest",
	ExternalTradeReference: "external_trade_reference",
	StatusCode:             "status_code",
	ExpiresAt:              "expires_at",
	SubmittedAt:            "submitted_at",
	ConfirmedAt:            "confirmed_at",
	ClosedAt:               "closed_at",
	ResultUnknown:          "result_unknown",
	FundingResultDigest:    "funding_result_digest",
	Version:                "version",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
}

// NewAttemptDao creates and returns a new DAO object for table data access.
func NewAttemptDao(handlers ...gdb.ModelHandler) *AttemptDao {
	return &AttemptDao{
		group:    "default",
		table:    "plugin_idc_payment_attempt",
		columns:  attemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AttemptDao) Columns() AttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
