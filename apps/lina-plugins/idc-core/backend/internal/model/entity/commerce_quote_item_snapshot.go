// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceQuoteItemSnapshot is the golang structure for table commerce_quote_item_snapshot.
type CommerceQuoteItemSnapshot struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	QuoteItemNo             string     `json:"quoteItemNo"             orm:"quote_item_no"             description:""`
	QuoteNo                 string     `json:"quoteNo"                 orm:"quote_no"                  description:""`
	CartNo                  *string    `json:"cartNo"                  orm:"cart_no"                   description:""`
	CartItemNo              *string    `json:"cartItemNo"              orm:"cart_item_no"              description:""`
	LineNumber              int        `json:"lineNumber"              orm:"line_number"               description:""`
	CurrencyCode            string     `json:"currencyCode"            orm:"currency_code"             description:""`
	ProductNo               string     `json:"productNo"               orm:"product_no"                description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                description:""`
	BillingRuleNo           string     `json:"billingRuleNo"           orm:"billing_rule_no"           description:""`
	ConfigurationSnapshotNo string     `json:"configurationSnapshotNo" orm:"configuration_snapshot_no" description:""`
	ProductNameZhCn         string     `json:"productNameZhCn"         orm:"product_name_zh_cn"        description:""`
	ProductNameEnUs         string     `json:"productNameEnUs"         orm:"product_name_en_us"        description:""`
	PurchaseCycles          int        `json:"purchaseCycles"          orm:"purchase_cycles"           description:""`
	ChargeTypeCode          string     `json:"chargeTypeCode"          orm:"charge_type_code"          description:""`
	PeriodUnitCode          string     `json:"periodUnitCode"          orm:"period_unit_code"          description:""`
	PeriodCount             int        `json:"periodCount"             orm:"period_count"              description:""`
	RequiresContract        bool       `json:"requiresContract"        orm:"requires_contract"         description:""`
	FulfillmentModeCode     string     `json:"fulfillmentModeCode"     orm:"fulfillment_mode_code"     description:""`
	RiskDecisionCode        string     `json:"riskDecisionCode"        orm:"risk_decision_code"        description:""`
	InventoryDecisionCode   string     `json:"inventoryDecisionCode"   orm:"inventory_decision_code"   description:""`
	RouteDecisionDigest     string     `json:"routeDecisionDigest"     orm:"route_decision_digest"     description:""`
	TaxPolicyNo             string     `json:"taxPolicyNo"             orm:"tax_policy_no"             description:""`
	PriceIncludesTax        bool       `json:"priceIncludesTax"        orm:"price_includes_tax"        description:""`
	TaxTreatmentCode        string     `json:"taxTreatmentCode"        orm:"tax_treatment_code"        description:""`
	TaxRateBasisPoints      *int       `json:"taxRateBasisPoints"      orm:"tax_rate_basis_points"     description:""`
	TaxCategoryCode         string     `json:"taxCategoryCode"         orm:"tax_category_code"         description:""`
	RoundingScopeCode       string     `json:"roundingScopeCode"       orm:"rounding_scope_code"       description:""`
	RoundingModeCode        string     `json:"roundingModeCode"        orm:"rounding_mode_code"        description:""`
	RoundingQuantumMinor    int64      `json:"roundingQuantumMinor"    orm:"rounding_quantum_minor"    description:""`
	SetupSubtotalMinor      int64      `json:"setupSubtotalMinor"      orm:"setup_subtotal_minor"      description:""`
	PeriodSubtotalMinor     int64      `json:"periodSubtotalMinor"     orm:"period_subtotal_minor"     description:""`
	SubtotalMinor           int64      `json:"subtotalMinor"           orm:"subtotal_minor"            description:""`
	DiscountMinor           int64      `json:"discountMinor"           orm:"discount_minor"            description:""`
	NetAmountMinor          int64      `json:"netAmountMinor"          orm:"net_amount_minor"          description:""`
	TaxAmountMinor          int64      `json:"taxAmountMinor"          orm:"tax_amount_minor"          description:""`
	TotalAmountMinor        int64      `json:"totalAmountMinor"        orm:"total_amount_minor"        description:""`
	ContentDigest           string     `json:"contentDigest"           orm:"content_digest"            description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	TrialLimitQuantity      int        `json:"trialLimitQuantity"      orm:"trial_limit_quantity"      description:""`
	PriorTrialUses          int        `json:"priorTrialUses"          orm:"prior_trial_uses"          description:""`
	ContractPolicyCode      string     `json:"contractPolicyCode"      orm:"contract_policy_code"      description:""`
	ContractTemplateNo      string     `json:"contractTemplateNo"      orm:"contract_template_no"      description:""`
}
