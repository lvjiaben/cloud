// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditStatementDao is the data access object for the table plugin_idc_core_credit_statement.
type CreditStatementDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  CreditStatementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// CreditStatementColumns defines and stores column names for the table plugin_idc_core_credit_statement.
type CreditStatementColumns struct {
	Id                     string //
	TenantId               string //
	StatementNo            string //
	CreditAccountNo        string //
	CustomerNo             string //
	CurrencyCode           string //
	TermsNo                string //
	TermsVersion           string //
	PeriodStartedAt        string //
	PeriodEndedAt          string //
	LineCount              string //
	PrincipalMinor         string //
	FeeMinor               string //
	AdjustmentMinor        string //
	TotalAmountMinor       string //
	PaidAmountMinor        string //
	WrittenOffAmountMinor  string //
	OutstandingAmountMinor string //
	StatusCode             string //
	ContentDigest          string //
	IssuedAt               string //
	PaymentDueAt           string //
	OverdueAt              string //
	SettledAt              string //
	CancelledAt            string //
	WrittenOffAt           string //
	Version                string //
	CreatedAt              string //
	UpdatedAt              string //
}

// creditStatementColumns holds the columns for the table plugin_idc_core_credit_statement.
var creditStatementColumns = CreditStatementColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	StatementNo:            "statement_no",
	CreditAccountNo:        "credit_account_no",
	CustomerNo:             "customer_no",
	CurrencyCode:           "currency_code",
	TermsNo:                "terms_no",
	TermsVersion:           "terms_version",
	PeriodStartedAt:        "period_started_at",
	PeriodEndedAt:          "period_ended_at",
	LineCount:              "line_count",
	PrincipalMinor:         "principal_minor",
	FeeMinor:               "fee_minor",
	AdjustmentMinor:        "adjustment_minor",
	TotalAmountMinor:       "total_amount_minor",
	PaidAmountMinor:        "paid_amount_minor",
	WrittenOffAmountMinor:  "written_off_amount_minor",
	OutstandingAmountMinor: "outstanding_amount_minor",
	StatusCode:             "status_code",
	ContentDigest:          "content_digest",
	IssuedAt:               "issued_at",
	PaymentDueAt:           "payment_due_at",
	OverdueAt:              "overdue_at",
	SettledAt:              "settled_at",
	CancelledAt:            "cancelled_at",
	WrittenOffAt:           "written_off_at",
	Version:                "version",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
}

// NewCreditStatementDao creates and returns a new DAO object for table data access.
func NewCreditStatementDao(handlers ...gdb.ModelHandler) *CreditStatementDao {
	return &CreditStatementDao{
		group:    "default",
		table:    "plugin_idc_core_credit_statement",
		columns:  creditStatementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditStatementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditStatementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditStatementDao) Columns() CreditStatementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditStatementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditStatementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditStatementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
