// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ExceptionFundActionDao is the data access object for the table plugin_idc_payment_exception_fund_action.
type ExceptionFundActionDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  ExceptionFundActionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// ExceptionFundActionColumns defines and stores column names for the table plugin_idc_payment_exception_fund_action.
type ExceptionFundActionColumns struct {
	Id              string //
	ActionNo        string //
	ExceptionNo     string //
	ActionCode      string //
	StatusCode      string //
	TargetOrderNo   string //
	TargetInvoiceNo string //
	AllocationNo    string //
	SettlementNo    string //
	RefundAttemptNo string //
	ProofReference  string //
	ProofDigest     string //
	RequestedBy     string //
	CompletedBy     string //
	RequestDigest   string //
	FailureCode     string //
	Version         string //
	RequestedAt     string //
	CompletedAt     string //
	UpdatedAt       string //
}

// exceptionFundActionColumns holds the columns for the table plugin_idc_payment_exception_fund_action.
var exceptionFundActionColumns = ExceptionFundActionColumns{
	Id:              "id",
	ActionNo:        "action_no",
	ExceptionNo:     "exception_no",
	ActionCode:      "action_code",
	StatusCode:      "status_code",
	TargetOrderNo:   "target_order_no",
	TargetInvoiceNo: "target_invoice_no",
	AllocationNo:    "allocation_no",
	SettlementNo:    "settlement_no",
	RefundAttemptNo: "refund_attempt_no",
	ProofReference:  "proof_reference",
	ProofDigest:     "proof_digest",
	RequestedBy:     "requested_by",
	CompletedBy:     "completed_by",
	RequestDigest:   "request_digest",
	FailureCode:     "failure_code",
	Version:         "version",
	RequestedAt:     "requested_at",
	CompletedAt:     "completed_at",
	UpdatedAt:       "updated_at",
}

// NewExceptionFundActionDao creates and returns a new DAO object for table data access.
func NewExceptionFundActionDao(handlers ...gdb.ModelHandler) *ExceptionFundActionDao {
	return &ExceptionFundActionDao{
		group:    "default",
		table:    "plugin_idc_payment_exception_fund_action",
		columns:  exceptionFundActionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ExceptionFundActionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ExceptionFundActionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ExceptionFundActionDao) Columns() ExceptionFundActionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ExceptionFundActionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ExceptionFundActionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ExceptionFundActionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
