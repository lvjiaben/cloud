// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingBenefitRule is the golang structure for table marketing_benefit_rule.
type MarketingBenefitRule struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	BenefitNo            string     `json:"benefitNo"            orm:"benefit_no"             description:""`
	CampaignNo           string     `json:"campaignNo"           orm:"campaign_no"            description:""`
	CampaignTypeCode     string     `json:"campaignTypeCode"     orm:"campaign_type_code"     description:""`
	BenefitTypeCode      string     `json:"benefitTypeCode"      orm:"benefit_type_code"      description:""`
	CurrencyCode         *string    `json:"currencyCode"         orm:"currency_code"          description:""`
	AmountMinor          *int64     `json:"amountMinor"          orm:"amount_minor"           description:""`
	RateBasisPoints      *int       `json:"rateBasisPoints"      orm:"rate_basis_points"      description:""`
	MaximumDiscountMinor *int64     `json:"maximumDiscountMinor" orm:"maximum_discount_minor" description:""`
	MinimumSpendMinor    *int64     `json:"minimumSpendMinor"    orm:"minimum_spend_minor"    description:""`
	PointsQuantity       *int64     `json:"pointsQuantity"       orm:"points_quantity"        description:""`
	PurchasePhaseCode    string     `json:"purchasePhaseCode"    orm:"purchase_phase_code"    description:""`
	CycleScopeCode       string     `json:"cycleScopeCode"       orm:"cycle_scope_code"       description:""`
	CycleCount           *int       `json:"cycleCount"           orm:"cycle_count"            description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
