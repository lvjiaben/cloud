// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditAccountDao is the data access object for the table plugin_idc_core_credit_account.
type CreditAccountDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  CreditAccountColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// CreditAccountColumns defines and stores column names for the table plugin_idc_core_credit_account.
type CreditAccountColumns struct {
	Id                       string //
	TenantId                 string //
	CreditAccountNo          string //
	CustomerNo               string //
	CurrencyCode             string //
	LedgerAccountNo          string //
	LedgerAccountTypeCode    string //
	OriginApplicationNo      string //
	CurrentTermsNo           string //
	CurrentTermsVersion      string //
	StatusCode               string //
	TotalLimitMinor          string //
	AvailableLimitMinor      string //
	UnbilledPrincipalMinor   string //
	BilledPrincipalMinor     string //
	DelinquentPrincipalMinor string //
	WrittenOffPrincipalMinor string //
	ValidFrom                string //
	ValidUntil               string //
	FreezeReasonCode         string //
	FrozenAt                 string //
	ClosedAt                 string //
	Version                  string //
	CreatedAt                string //
	UpdatedAt                string //
}

// creditAccountColumns holds the columns for the table plugin_idc_core_credit_account.
var creditAccountColumns = CreditAccountColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	CreditAccountNo:          "credit_account_no",
	CustomerNo:               "customer_no",
	CurrencyCode:             "currency_code",
	LedgerAccountNo:          "ledger_account_no",
	LedgerAccountTypeCode:    "ledger_account_type_code",
	OriginApplicationNo:      "origin_application_no",
	CurrentTermsNo:           "current_terms_no",
	CurrentTermsVersion:      "current_terms_version",
	StatusCode:               "status_code",
	TotalLimitMinor:          "total_limit_minor",
	AvailableLimitMinor:      "available_limit_minor",
	UnbilledPrincipalMinor:   "unbilled_principal_minor",
	BilledPrincipalMinor:     "billed_principal_minor",
	DelinquentPrincipalMinor: "delinquent_principal_minor",
	WrittenOffPrincipalMinor: "written_off_principal_minor",
	ValidFrom:                "valid_from",
	ValidUntil:               "valid_until",
	FreezeReasonCode:         "freeze_reason_code",
	FrozenAt:                 "frozen_at",
	ClosedAt:                 "closed_at",
	Version:                  "version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewCreditAccountDao creates and returns a new DAO object for table data access.
func NewCreditAccountDao(handlers ...gdb.ModelHandler) *CreditAccountDao {
	return &CreditAccountDao{
		group:    "default",
		table:    "plugin_idc_core_credit_account",
		columns:  creditAccountColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditAccountDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditAccountDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditAccountDao) Columns() CreditAccountColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditAccountDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditAccountDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditAccountDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
