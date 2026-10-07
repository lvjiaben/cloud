// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogBillingRule is the golang structure for table catalog_billing_rule.
type CatalogBillingRule struct {
	Id                      int64      `json:"id"                      orm:"id"                         description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                  description:""`
	BillingRuleNo           string     `json:"billingRuleNo"           orm:"billing_rule_no"            description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                 description:""`
	RuleCode                string     `json:"ruleCode"                orm:"rule_code"                  description:""`
	ChargeTypeCode          string     `json:"chargeTypeCode"          orm:"charge_type_code"           description:""`
	PeriodUnitCode          string     `json:"periodUnitCode"          orm:"period_unit_code"           description:""`
	PeriodCount             int        `json:"periodCount"             orm:"period_count"               description:""`
	PricePhaseCode          string     `json:"pricePhaseCode"          orm:"price_phase_code"           description:""`
	QuantityModeCode        string     `json:"quantityModeCode"        orm:"quantity_mode_code"         description:""`
	TrialQuantity           int        `json:"trialQuantity"           orm:"trial_quantity"             description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"                description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                 description:""`
	MinimumPurchaseCycles   int        `json:"minimumPurchaseCycles"   orm:"minimum_purchase_cycles"    description:"Minimum number of this explicit billing period accepted for a new purchase"`
	MaximumRenewalCycles    *int       `json:"maximumRenewalCycles"    orm:"maximum_renewal_cycles"     description:"Optional maximum number of this explicit billing period accepted per renewal quote"`
	MeterUnitCode           *string    `json:"meterUnitCode"           orm:"meter_unit_code"            description:"Merchant-defined normalized authoritative meter unit for usage billing"`
	TierModeCode            *string    `json:"tierModeCode"            orm:"tier_mode_code"             description:""`
	UsageCollectionUnitCode *string    `json:"usageCollectionUnitCode" orm:"usage_collection_unit_code" description:""`
	UsageCollectionCount    *int       `json:"usageCollectionCount"    orm:"usage_collection_count"     description:""`
	UsageRoundingModeCode   *string    `json:"usageRoundingModeCode"   orm:"usage_rounding_mode_code"   description:""`
	UsageRoundingQuantity   *int64     `json:"usageRoundingQuantity"   orm:"usage_rounding_quantity"    description:""`
	UsageMinimumQuantity    *int64     `json:"usageMinimumQuantity"    orm:"usage_minimum_quantity"     description:""`
	UsageMaximumQuantity    *int64     `json:"usageMaximumQuantity"    orm:"usage_maximum_quantity"     description:""`
	UsageMissingPolicyCode  *string    `json:"usageMissingPolicyCode"  orm:"usage_missing_policy_code"  description:"Fail-closed pending recalculation or explicit previous-period estimate policy"`
}
