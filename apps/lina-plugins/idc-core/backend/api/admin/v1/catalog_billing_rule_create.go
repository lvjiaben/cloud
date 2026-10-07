// This file defines staff billing-rule append operations.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogBillingRuleCreateReq struct {
	g.Meta                 `path:"/admin/catalog/product-versions/{versionNo}/billing-rules" method:"post" tags:"IDC Catalog Management" summary:"Create catalog billing rule" dc:"Append one validated billing model to an editable manual, copy, or migration draft." permission:"idc:admin:catalog:billing:write"`
	VersionNo              string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable editable draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	RuleCode               string `json:"ruleCode" v:"required|max-length:64" dc:"Stable version-local billing code" eg:"monthly"`
	ChargeType             string `json:"chargeType" v:"required|in:one_time,recurring,tiered,usage,trial" dc:"Billing model" eg:"recurring"`
	PeriodUnit             string `json:"periodUnit" v:"required|in:once,hour,day,month,year" dc:"Explicit billing period unit" eg:"month"`
	PeriodCount            int    `json:"periodCount" v:"between:1,87600" dc:"Positive count of period units" eg:"1"`
	PricePhase             string `json:"pricePhase" v:"required|in:new_purchase,renewal,both" dc:"Applicable purchase phase" eg:"both"`
	QuantityMode           string `json:"quantityMode" v:"required|in:fixed,per_unit,tiered,usage" dc:"Quantity pricing model" eg:"fixed"`
	TrialQuantity          int    `json:"trialQuantity" v:"min:0|max:100" dc:"Authoritative allowed trial uses" eg:"0"`
	MinimumPurchaseCycles  int    `json:"minimumPurchaseCycles" v:"between:1,120" dc:"Minimum new-purchase cycles" eg:"1"`
	MaximumRenewalCycles   *int   `json:"maximumRenewalCycles" dc:"Optional maximum cycles in one renewal quote" eg:"12"`
	MeterUnit              string `json:"meterUnit" v:"max-length:64" dc:"Usage meter unit code" eg:"gb_transfer"`
	TierMode               string `json:"tierMode" v:"max-length:16" dc:"Tier calculation mode" eg:"graduated"`
	UsageCollectionUnit    string `json:"usageCollectionUnit" v:"max-length:16" dc:"Usage collection unit" eg:"hour"`
	UsageCollectionCount   *int   `json:"usageCollectionCount" dc:"Usage collection unit count" eg:"1"`
	UsageRoundingMode      string `json:"usageRoundingMode" v:"max-length:16" dc:"Usage integer rounding mode" eg:"up"`
	UsageRoundingQuantity  *int64 `json:"usageRoundingQuantity" dc:"Positive usage rounding quantum" eg:"1"`
	UsageMinimumQuantity   *int64 `json:"usageMinimumQuantity" dc:"Nonnegative billed usage floor" eg:"0"`
	UsageMaximumQuantity   *int64 `json:"usageMaximumQuantity" dc:"Optional billed usage cap" eg:"1000"`
	UsageMissingDataPolicy string `json:"usageMissingDataPolicy" v:"max-length:32" dc:"Missing authoritative usage policy" eg:"pending_recalculation"`
	Status                 string `json:"status" v:"required|in:active,inactive" dc:"Draft billing-rule status" eg:"active"`
}

type CatalogBillingRuleCreateRes struct {
	BillingRuleNo string `json:"billingRuleNo" dc:"Stable billing-rule business number" eg:"bil_0123456789abcdef0123456789abcdef"`
	VersionNo     string `json:"versionNo" dc:"Owning version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	RuleCode      string `json:"ruleCode" dc:"Version-local billing code" eg:"monthly"`
	ChargeType    string `json:"chargeType" dc:"Billing model" eg:"recurring"`
	PeriodUnit    string `json:"periodUnit" dc:"Explicit billing period unit" eg:"month"`
	PeriodCount   int    `json:"periodCount" dc:"Positive count of period units" eg:"1"`
	PricePhase    string `json:"pricePhase" dc:"Applicable purchase phase" eg:"both"`
	QuantityMode  string `json:"quantityMode" dc:"Quantity pricing model" eg:"fixed"`
	Status        string `json:"status" dc:"Stable billing-rule status" eg:"active"`
}
