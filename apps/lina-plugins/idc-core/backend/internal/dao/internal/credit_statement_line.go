// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditStatementLineDao is the data access object for the table plugin_idc_core_credit_statement_line.
type CreditStatementLineDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  CreditStatementLineColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// CreditStatementLineColumns defines and stores column names for the table plugin_idc_core_credit_statement_line.
type CreditStatementLineColumns struct {
	Id              string //
	TenantId        string //
	StatementLineNo string //
	StatementNo     string //
	CreditAccountNo string //
	CustomerNo      string //
	CurrencyCode    string //
	LineNumber      string //
	LineTypeCode    string //
	ExposureNo      string //
	SourceTypeCode  string //
	SourceReference string //
	AmountMinor     string //
	ContentDigest   string //
	CreatedAt       string //
}

// creditStatementLineColumns holds the columns for the table plugin_idc_core_credit_statement_line.
var creditStatementLineColumns = CreditStatementLineColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	StatementLineNo: "statement_line_no",
	StatementNo:     "statement_no",
	CreditAccountNo: "credit_account_no",
	CustomerNo:      "customer_no",
	CurrencyCode:    "currency_code",
	LineNumber:      "line_number",
	LineTypeCode:    "line_type_code",
	ExposureNo:      "exposure_no",
	SourceTypeCode:  "source_type_code",
	SourceReference: "source_reference",
	AmountMinor:     "amount_minor",
	ContentDigest:   "content_digest",
	CreatedAt:       "created_at",
}

// NewCreditStatementLineDao creates and returns a new DAO object for table data access.
func NewCreditStatementLineDao(handlers ...gdb.ModelHandler) *CreditStatementLineDao {
	return &CreditStatementLineDao{
		group:    "default",
		table:    "plugin_idc_core_credit_statement_line",
		columns:  creditStatementLineColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditStatementLineDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditStatementLineDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditStatementLineDao) Columns() CreditStatementLineColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditStatementLineDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditStatementLineDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditStatementLineDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
