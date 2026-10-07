// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditPolicyRevisionDao is the data access object for the table plugin_idc_core_credit_policy_revision.
type CreditPolicyRevisionDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  CreditPolicyRevisionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// CreditPolicyRevisionColumns defines and stores column names for the table plugin_idc_core_credit_policy_revision.
type CreditPolicyRevisionColumns struct {
	Id                       string //
	TenantId                 string //
	PolicyNo                 string //
	PolicyVersion            string //
	CurrencyCode             string //
	MaximumLimitMinor        string //
	DualReviewThresholdMinor string //
	BillingDay               string //
	RepaymentPeriodDays      string //
	GracePeriodDays          string //
	DailyLateFeeBasisPoints  string //
	MaximumLateFeeMinor      string //
	ConfiguredBy             string //
	ContentDigest            string //
	CreatedAt                string //
}

// creditPolicyRevisionColumns holds the columns for the table plugin_idc_core_credit_policy_revision.
var creditPolicyRevisionColumns = CreditPolicyRevisionColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	PolicyNo:                 "policy_no",
	PolicyVersion:            "policy_version",
	CurrencyCode:             "currency_code",
	MaximumLimitMinor:        "maximum_limit_minor",
	DualReviewThresholdMinor: "dual_review_threshold_minor",
	BillingDay:               "billing_day",
	RepaymentPeriodDays:      "repayment_period_days",
	GracePeriodDays:          "grace_period_days",
	DailyLateFeeBasisPoints:  "daily_late_fee_basis_points",
	MaximumLateFeeMinor:      "maximum_late_fee_minor",
	ConfiguredBy:             "configured_by",
	ContentDigest:            "content_digest",
	CreatedAt:                "created_at",
}

// NewCreditPolicyRevisionDao creates and returns a new DAO object for table data access.
func NewCreditPolicyRevisionDao(handlers ...gdb.ModelHandler) *CreditPolicyRevisionDao {
	return &CreditPolicyRevisionDao{
		group:    "default",
		table:    "plugin_idc_core_credit_policy_revision",
		columns:  creditPolicyRevisionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditPolicyRevisionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditPolicyRevisionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditPolicyRevisionDao) Columns() CreditPolicyRevisionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditPolicyRevisionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditPolicyRevisionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditPolicyRevisionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
