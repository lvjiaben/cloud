// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditWriteoffDao is the data access object for the table plugin_idc_core_credit_writeoff.
type CreditWriteoffDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  CreditWriteoffColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// CreditWriteoffColumns defines and stores column names for the table plugin_idc_core_credit_writeoff.
type CreditWriteoffColumns struct {
	Id                     string //
	TenantId               string //
	WriteoffNo             string //
	StatementNo            string //
	CreditAccountNo        string //
	CustomerNo             string //
	CurrencyCode           string //
	WriteoffTypeCode       string //
	AmountMinor            string //
	OriginalWriteoffNo     string //
	LedgerTransactionNo    string //
	ReasonCode             string //
	RequestedBy            string //
	ReviewedBy             string //
	ApprovalEvidenceDigest string //
	ContentDigest          string //
	PostedAt               string //
	CreatedAt              string //
}

// creditWriteoffColumns holds the columns for the table plugin_idc_core_credit_writeoff.
var creditWriteoffColumns = CreditWriteoffColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	WriteoffNo:             "writeoff_no",
	StatementNo:            "statement_no",
	CreditAccountNo:        "credit_account_no",
	CustomerNo:             "customer_no",
	CurrencyCode:           "currency_code",
	WriteoffTypeCode:       "writeoff_type_code",
	AmountMinor:            "amount_minor",
	OriginalWriteoffNo:     "original_writeoff_no",
	LedgerTransactionNo:    "ledger_transaction_no",
	ReasonCode:             "reason_code",
	RequestedBy:            "requested_by",
	ReviewedBy:             "reviewed_by",
	ApprovalEvidenceDigest: "approval_evidence_digest",
	ContentDigest:          "content_digest",
	PostedAt:               "posted_at",
	CreatedAt:              "created_at",
}

// NewCreditWriteoffDao creates and returns a new DAO object for table data access.
func NewCreditWriteoffDao(handlers ...gdb.ModelHandler) *CreditWriteoffDao {
	return &CreditWriteoffDao{
		group:    "default",
		table:    "plugin_idc_core_credit_writeoff",
		columns:  creditWriteoffColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditWriteoffDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditWriteoffDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditWriteoffDao) Columns() CreditWriteoffColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditWriteoffDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditWriteoffDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditWriteoffDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
