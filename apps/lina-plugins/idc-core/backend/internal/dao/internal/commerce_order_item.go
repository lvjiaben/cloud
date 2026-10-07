// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderItemDao is the data access object for the table plugin_idc_core_commerce_order_item.
type CommerceOrderItemDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  CommerceOrderItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// CommerceOrderItemColumns defines and stores column names for the table plugin_idc_core_commerce_order_item.
type CommerceOrderItemColumns struct {
	Id                      string //
	TenantId                string //
	OrderItemNo             string //
	OrderNo                 string //
	QuoteNo                 string //
	QuoteItemNo             string //
	LineNumber              string //
	CurrencyCode            string //
	ProductNo               string //
	VersionNo               string //
	BillingRuleNo           string //
	ConfigurationSnapshotNo string //
	ProductNameZhCn         string //
	ProductNameEnUs         string //
	PurchaseCycles          string //
	ChargeTypeCode          string //
	PeriodUnitCode          string //
	PeriodCount             string //
	RequiresContract        string //
	FulfillmentModeCode     string //
	RiskDecisionCode        string //
	InventoryDecisionCode   string //
	RouteDecisionDigest     string //
	TaxPolicyNo             string //
	PriceIncludesTax        string //
	TaxTreatmentCode        string //
	TaxRateBasisPoints      string //
	TaxCategoryCode         string //
	RoundingScopeCode       string //
	RoundingModeCode        string //
	RoundingQuantumMinor    string //
	SetupSubtotalMinor      string //
	PeriodSubtotalMinor     string //
	SubtotalMinor           string //
	DiscountMinor           string //
	NetAmountMinor          string //
	TaxAmountMinor          string //
	TotalAmountMinor        string //
	ContentDigest           string //
	CreatedAt               string //
	TrialLimitQuantity      string //
	PriorTrialUses          string //
	ContractPolicyCode      string //
	ContractTemplateNo      string //
}

// commerceOrderItemColumns holds the columns for the table plugin_idc_core_commerce_order_item.
var commerceOrderItemColumns = CommerceOrderItemColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	OrderItemNo:             "order_item_no",
	OrderNo:                 "order_no",
	QuoteNo:                 "quote_no",
	QuoteItemNo:             "quote_item_no",
	LineNumber:              "line_number",
	CurrencyCode:            "currency_code",
	ProductNo:               "product_no",
	VersionNo:               "version_no",
	BillingRuleNo:           "billing_rule_no",
	ConfigurationSnapshotNo: "configuration_snapshot_no",
	ProductNameZhCn:         "product_name_zh_cn",
	ProductNameEnUs:         "product_name_en_us",
	PurchaseCycles:          "purchase_cycles",
	ChargeTypeCode:          "charge_type_code",
	PeriodUnitCode:          "period_unit_code",
	PeriodCount:             "period_count",
	RequiresContract:        "requires_contract",
	FulfillmentModeCode:     "fulfillment_mode_code",
	RiskDecisionCode:        "risk_decision_code",
	InventoryDecisionCode:   "inventory_decision_code",
	RouteDecisionDigest:     "route_decision_digest",
	TaxPolicyNo:             "tax_policy_no",
	PriceIncludesTax:        "price_includes_tax",
	TaxTreatmentCode:        "tax_treatment_code",
	TaxRateBasisPoints:      "tax_rate_basis_points",
	TaxCategoryCode:         "tax_category_code",
	RoundingScopeCode:       "rounding_scope_code",
	RoundingModeCode:        "rounding_mode_code",
	RoundingQuantumMinor:    "rounding_quantum_minor",
	SetupSubtotalMinor:      "setup_subtotal_minor",
	PeriodSubtotalMinor:     "period_subtotal_minor",
	SubtotalMinor:           "subtotal_minor",
	DiscountMinor:           "discount_minor",
	NetAmountMinor:          "net_amount_minor",
	TaxAmountMinor:          "tax_amount_minor",
	TotalAmountMinor:        "total_amount_minor",
	ContentDigest:           "content_digest",
	CreatedAt:               "created_at",
	TrialLimitQuantity:      "trial_limit_quantity",
	PriorTrialUses:          "prior_trial_uses",
	ContractPolicyCode:      "contract_policy_code",
	ContractTemplateNo:      "contract_template_no",
}

// NewCommerceOrderItemDao creates and returns a new DAO object for table data access.
func NewCommerceOrderItemDao(handlers ...gdb.ModelHandler) *CommerceOrderItemDao {
	return &CommerceOrderItemDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_order_item",
		columns:  commerceOrderItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceOrderItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceOrderItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceOrderItemDao) Columns() CommerceOrderItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceOrderItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceOrderItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceOrderItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
