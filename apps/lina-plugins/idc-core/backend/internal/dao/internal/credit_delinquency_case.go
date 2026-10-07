// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditDelinquencyCaseDao is the data access object for the table plugin_idc_core_credit_delinquency_case.
type CreditDelinquencyCaseDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  CreditDelinquencyCaseColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// CreditDelinquencyCaseColumns defines and stores column names for the table plugin_idc_core_credit_delinquency_case.
type CreditDelinquencyCaseColumns struct {
	Id                       string //
	TenantId                 string //
	CaseNo                   string //
	StatementNo              string //
	CreditAccountNo          string //
	CustomerNo               string //
	CurrencyCode             string //
	TermsNo                  string //
	TermsVersion             string //
	StatusCode               string //
	PrincipalMinor           string //
	LateFeeMinor             string //
	OutstandingAmountMinor   string //
	OpenedAt                 string //
	GraceUntil               string //
	FrozenAt                 string //
	ServiceActionRequestedAt string //
	WrittenOffAt             string //
	ResolvedAt               string //
	ReasonCode               string //
	SourceSnapshotDigest     string //
	Version                  string //
	CreatedAt                string //
	UpdatedAt                string //
}

// creditDelinquencyCaseColumns holds the columns for the table plugin_idc_core_credit_delinquency_case.
var creditDelinquencyCaseColumns = CreditDelinquencyCaseColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	CaseNo:                   "case_no",
	StatementNo:              "statement_no",
	CreditAccountNo:          "credit_account_no",
	CustomerNo:               "customer_no",
	CurrencyCode:             "currency_code",
	TermsNo:                  "terms_no",
	TermsVersion:             "terms_version",
	StatusCode:               "status_code",
	PrincipalMinor:           "principal_minor",
	LateFeeMinor:             "late_fee_minor",
	OutstandingAmountMinor:   "outstanding_amount_minor",
	OpenedAt:                 "opened_at",
	GraceUntil:               "grace_until",
	FrozenAt:                 "frozen_at",
	ServiceActionRequestedAt: "service_action_requested_at",
	WrittenOffAt:             "written_off_at",
	ResolvedAt:               "resolved_at",
	ReasonCode:               "reason_code",
	SourceSnapshotDigest:     "source_snapshot_digest",
	Version:                  "version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewCreditDelinquencyCaseDao creates and returns a new DAO object for table data access.
func NewCreditDelinquencyCaseDao(handlers ...gdb.ModelHandler) *CreditDelinquencyCaseDao {
	return &CreditDelinquencyCaseDao{
		group:    "default",
		table:    "plugin_idc_core_credit_delinquency_case",
		columns:  creditDelinquencyCaseColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditDelinquencyCaseDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditDelinquencyCaseDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditDelinquencyCaseDao) Columns() CreditDelinquencyCaseColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditDelinquencyCaseDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditDelinquencyCaseDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditDelinquencyCaseDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
