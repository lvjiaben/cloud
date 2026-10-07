// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingAdjustmentPolicyDao is the data access object for the table plugin_idc_core_billing_adjustment_policy.
type BillingAdjustmentPolicyDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  BillingAdjustmentPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// BillingAdjustmentPolicyColumns defines and stores column names for the table plugin_idc_core_billing_adjustment_policy.
type BillingAdjustmentPolicyColumns struct {
	Id                   string //
	TenantId             string //
	PolicyNo             string //
	CurrencyCode         string //
	ReviewThresholdMinor string //
	Version              string //
	ConfiguredBy         string //
	ContentDigest        string //
	CreatedAt            string //
}

// billingAdjustmentPolicyColumns holds the columns for the table plugin_idc_core_billing_adjustment_policy.
var billingAdjustmentPolicyColumns = BillingAdjustmentPolicyColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	PolicyNo:             "policy_no",
	CurrencyCode:         "currency_code",
	ReviewThresholdMinor: "review_threshold_minor",
	Version:              "version",
	ConfiguredBy:         "configured_by",
	ContentDigest:        "content_digest",
	CreatedAt:            "created_at",
}

// NewBillingAdjustmentPolicyDao creates and returns a new DAO object for table data access.
func NewBillingAdjustmentPolicyDao(handlers ...gdb.ModelHandler) *BillingAdjustmentPolicyDao {
	return &BillingAdjustmentPolicyDao{
		group:    "default",
		table:    "plugin_idc_core_billing_adjustment_policy",
		columns:  billingAdjustmentPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingAdjustmentPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingAdjustmentPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingAdjustmentPolicyDao) Columns() BillingAdjustmentPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingAdjustmentPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingAdjustmentPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingAdjustmentPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
