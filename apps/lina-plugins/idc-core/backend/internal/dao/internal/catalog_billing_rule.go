// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogBillingRuleDao is the data access object for the table plugin_idc_core_catalog_billing_rule.
type CatalogBillingRuleDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  CatalogBillingRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// CatalogBillingRuleColumns defines and stores column names for the table plugin_idc_core_catalog_billing_rule.
type CatalogBillingRuleColumns struct {
	Id                      string //
	TenantId                string //
	BillingRuleNo           string //
	VersionNo               string //
	RuleCode                string //
	ChargeTypeCode          string //
	PeriodUnitCode          string //
	PeriodCount             string //
	PricePhaseCode          string //
	QuantityModeCode        string //
	TrialQuantity           string //
	StatusCode              string //
	CreatedAt               string //
	MinimumPurchaseCycles   string // Minimum number of this explicit billing period accepted for a new purchase
	MaximumRenewalCycles    string // Optional maximum number of this explicit billing period accepted per renewal quote
	MeterUnitCode           string // Merchant-defined normalized authoritative meter unit for usage billing
	TierModeCode            string //
	UsageCollectionUnitCode string //
	UsageCollectionCount    string //
	UsageRoundingModeCode   string //
	UsageRoundingQuantity   string //
	UsageMinimumQuantity    string //
	UsageMaximumQuantity    string //
	UsageMissingPolicyCode  string // Fail-closed pending recalculation or explicit previous-period estimate policy
}

// catalogBillingRuleColumns holds the columns for the table plugin_idc_core_catalog_billing_rule.
var catalogBillingRuleColumns = CatalogBillingRuleColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	BillingRuleNo:           "billing_rule_no",
	VersionNo:               "version_no",
	RuleCode:                "rule_code",
	ChargeTypeCode:          "charge_type_code",
	PeriodUnitCode:          "period_unit_code",
	PeriodCount:             "period_count",
	PricePhaseCode:          "price_phase_code",
	QuantityModeCode:        "quantity_mode_code",
	TrialQuantity:           "trial_quantity",
	StatusCode:              "status_code",
	CreatedAt:               "created_at",
	MinimumPurchaseCycles:   "minimum_purchase_cycles",
	MaximumRenewalCycles:    "maximum_renewal_cycles",
	MeterUnitCode:           "meter_unit_code",
	TierModeCode:            "tier_mode_code",
	UsageCollectionUnitCode: "usage_collection_unit_code",
	UsageCollectionCount:    "usage_collection_count",
	UsageRoundingModeCode:   "usage_rounding_mode_code",
	UsageRoundingQuantity:   "usage_rounding_quantity",
	UsageMinimumQuantity:    "usage_minimum_quantity",
	UsageMaximumQuantity:    "usage_maximum_quantity",
	UsageMissingPolicyCode:  "usage_missing_policy_code",
}

// NewCatalogBillingRuleDao creates and returns a new DAO object for table data access.
func NewCatalogBillingRuleDao(handlers ...gdb.ModelHandler) *CatalogBillingRuleDao {
	return &CatalogBillingRuleDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_billing_rule",
		columns:  catalogBillingRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogBillingRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogBillingRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogBillingRuleDao) Columns() CatalogBillingRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogBillingRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogBillingRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogBillingRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
