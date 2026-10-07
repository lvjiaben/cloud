// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentPriceRule is the golang structure for table sales_agent_price_rule.
type SalesAgentPriceRule struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	PriceRuleNo         string     `json:"priceRuleNo"         orm:"price_rule_no"         description:""`
	ProductRuleNo       string     `json:"productRuleNo"       orm:"product_rule_no"       description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	AdjustmentTypeCode  string     `json:"adjustmentTypeCode"  orm:"adjustment_type_code"  description:""`
	FixedPriceMinor     *int64     `json:"fixedPriceMinor"     orm:"fixed_price_minor"     description:""`
	DiscountBasisPoints *int       `json:"discountBasisPoints" orm:"discount_basis_points" description:""`
	MinimumMarginMinor  int64      `json:"minimumMarginMinor"  orm:"minimum_margin_minor"  description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	EffectiveAt         *time.Time `json:"effectiveAt"         orm:"effective_at"          description:""`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"            description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	RuleDigest          string     `json:"ruleDigest"          orm:"rule_digest"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
