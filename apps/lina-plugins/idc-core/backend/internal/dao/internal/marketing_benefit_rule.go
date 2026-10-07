// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingBenefitRuleDao is the data access object for the table plugin_idc_core_marketing_benefit_rule.
type MarketingBenefitRuleDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  MarketingBenefitRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// MarketingBenefitRuleColumns defines and stores column names for the table plugin_idc_core_marketing_benefit_rule.
type MarketingBenefitRuleColumns struct {
	Id                   string //
	TenantId             string //
	BenefitNo            string //
	CampaignNo           string //
	CampaignTypeCode     string //
	BenefitTypeCode      string //
	CurrencyCode         string //
	AmountMinor          string //
	RateBasisPoints      string //
	MaximumDiscountMinor string //
	MinimumSpendMinor    string //
	PointsQuantity       string //
	PurchasePhaseCode    string //
	CycleScopeCode       string //
	CycleCount           string //
	StatusCode           string //
	Version              string //
	CreatedAt            string //
	UpdatedAt            string //
}

// marketingBenefitRuleColumns holds the columns for the table plugin_idc_core_marketing_benefit_rule.
var marketingBenefitRuleColumns = MarketingBenefitRuleColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	BenefitNo:            "benefit_no",
	CampaignNo:           "campaign_no",
	CampaignTypeCode:     "campaign_type_code",
	BenefitTypeCode:      "benefit_type_code",
	CurrencyCode:         "currency_code",
	AmountMinor:          "amount_minor",
	RateBasisPoints:      "rate_basis_points",
	MaximumDiscountMinor: "maximum_discount_minor",
	MinimumSpendMinor:    "minimum_spend_minor",
	PointsQuantity:       "points_quantity",
	PurchasePhaseCode:    "purchase_phase_code",
	CycleScopeCode:       "cycle_scope_code",
	CycleCount:           "cycle_count",
	StatusCode:           "status_code",
	Version:              "version",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewMarketingBenefitRuleDao creates and returns a new DAO object for table data access.
func NewMarketingBenefitRuleDao(handlers ...gdb.ModelHandler) *MarketingBenefitRuleDao {
	return &MarketingBenefitRuleDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_benefit_rule",
		columns:  marketingBenefitRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingBenefitRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingBenefitRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingBenefitRuleDao) Columns() MarketingBenefitRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingBenefitRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingBenefitRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingBenefitRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
