// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditAccountTermsDao is the data access object for the table plugin_idc_core_credit_account_terms.
type CreditAccountTermsDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  CreditAccountTermsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// CreditAccountTermsColumns defines and stores column names for the table plugin_idc_core_credit_account_terms.
type CreditAccountTermsColumns struct {
	Id                      string //
	TenantId                string //
	TermsNo                 string //
	CreditAccountNo         string //
	CustomerNo              string //
	CurrencyCode            string //
	TermsVersion            string //
	ApplicationNo           string //
	PolicyNo                string //
	PolicyVersion           string //
	TotalLimitMinor         string //
	BillingDay              string //
	RepaymentPeriodDays     string //
	GracePeriodDays         string //
	DailyLateFeeBasisPoints string //
	MaximumLateFeeMinor     string //
	EffectiveFrom           string //
	EffectiveUntil          string //
	ApprovedBy              string //
	ApprovalDigest          string //
	ContentDigest           string //
	CreatedAt               string //
}

// creditAccountTermsColumns holds the columns for the table plugin_idc_core_credit_account_terms.
var creditAccountTermsColumns = CreditAccountTermsColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	TermsNo:                 "terms_no",
	CreditAccountNo:         "credit_account_no",
	CustomerNo:              "customer_no",
	CurrencyCode:            "currency_code",
	TermsVersion:            "terms_version",
	ApplicationNo:           "application_no",
	PolicyNo:                "policy_no",
	PolicyVersion:           "policy_version",
	TotalLimitMinor:         "total_limit_minor",
	BillingDay:              "billing_day",
	RepaymentPeriodDays:     "repayment_period_days",
	GracePeriodDays:         "grace_period_days",
	DailyLateFeeBasisPoints: "daily_late_fee_basis_points",
	MaximumLateFeeMinor:     "maximum_late_fee_minor",
	EffectiveFrom:           "effective_from",
	EffectiveUntil:          "effective_until",
	ApprovedBy:              "approved_by",
	ApprovalDigest:          "approval_digest",
	ContentDigest:           "content_digest",
	CreatedAt:               "created_at",
}

// NewCreditAccountTermsDao creates and returns a new DAO object for table data access.
func NewCreditAccountTermsDao(handlers ...gdb.ModelHandler) *CreditAccountTermsDao {
	return &CreditAccountTermsDao{
		group:    "default",
		table:    "plugin_idc_core_credit_account_terms",
		columns:  creditAccountTermsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditAccountTermsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditAccountTermsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditAccountTermsDao) Columns() CreditAccountTermsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditAccountTermsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditAccountTermsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditAccountTermsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
