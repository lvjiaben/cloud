// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ExceptionFundDao is the data access object for the table plugin_idc_payment_exception_fund.
type ExceptionFundDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  ExceptionFundColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// ExceptionFundColumns defines and stores column names for the table plugin_idc_payment_exception_fund.
type ExceptionFundColumns struct {
	Id             string //
	ExceptionNo    string //
	AttemptNo      string //
	ChannelNo      string //
	CurrencyCode   string //
	AmountMinor    string //
	ReasonCode     string //
	StatusCode     string //
	EvidenceDigest string //
	CreatedAt      string //
	ResolvedAt     string //
}

// exceptionFundColumns holds the columns for the table plugin_idc_payment_exception_fund.
var exceptionFundColumns = ExceptionFundColumns{
	Id:             "id",
	ExceptionNo:    "exception_no",
	AttemptNo:      "attempt_no",
	ChannelNo:      "channel_no",
	CurrencyCode:   "currency_code",
	AmountMinor:    "amount_minor",
	ReasonCode:     "reason_code",
	StatusCode:     "status_code",
	EvidenceDigest: "evidence_digest",
	CreatedAt:      "created_at",
	ResolvedAt:     "resolved_at",
}

// NewExceptionFundDao creates and returns a new DAO object for table data access.
func NewExceptionFundDao(handlers ...gdb.ModelHandler) *ExceptionFundDao {
	return &ExceptionFundDao{
		group:    "default",
		table:    "plugin_idc_payment_exception_fund",
		columns:  exceptionFundColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ExceptionFundDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ExceptionFundDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ExceptionFundDao) Columns() ExceptionFundColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ExceptionFundDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ExceptionFundDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ExceptionFundDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
