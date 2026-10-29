// This file defines atomic creation of one normal billing period and its first
// complete base price. Trial and metered models retain their dedicated contracts.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogPricedBillingRuleCreateReq struct {
	g.Meta                `path:"/admin/catalog/product-versions/{versionNo}/priced-billing-rules" method:"post" tags:"IDC Catalog Management" summary:"Create billing period with base price" dc:"Save one ordinary fixed-price billing period and its first currency price together. Billing, price and product-save permissions are enforced; failure changes neither the billing rule nor its price." permission:"idc:admin:catalog:billing:write"`
	VersionNo             string  `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Editable product version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	RuleCode              string  `json:"ruleCode" v:"required|max-length:64" dc:"Stable version-local billing code reused when retrying the same creation" eg:"monthly"`
	PeriodUnit            string  `json:"periodUnit" v:"required|in:once,hour,day,month,year" dc:"Billing period unit: once, hour, day, month or year; once creates a one-time charge, others create recurring charges" eg:"month"`
	PeriodCount           int     `json:"periodCount" v:"between:1,87600" dc:"Units per period; must equal one for a one-time charge" eg:"1"`
	MinimumPurchaseCycles int     `json:"minimumPurchaseCycles" d:"1" v:"between:1,120" dc:"Minimum purchase cycles; one for a one-time charge" eg:"1"`
	MaximumRenewalCycles  *int    `json:"maximumRenewalCycles" dc:"Optional maximum cycles per renewal; absent means no additional cap and is required to be absent for one-time charges" eg:"12"`
	CurrencyCode          string  `json:"currencyCode" v:"required|length:3,3" dc:"Explicit uppercase ISO currency code; no conversion or price inference" eg:"CNY"`
	AmountMinor           string  `json:"amountMinor" v:"required|regex:^\\d{1,19}$" dc:"Exact new-purchase base amount in minor units; zero explicitly means free" eg:"1100"`
	SetupFeeMinor         string  `json:"setupFeeMinor" v:"required|regex:^\\d{1,19}$" dc:"Exact additional one-time setup fee in minor units" eg:"0"`
	RenewalAmountMinor    *string `json:"renewalAmountMinor" v:"regex:^\\d{1,19}$" dc:"Explicit recurring renewal amount in minor units; required for recurring periods, absent for one-time charges" eg:"1100"`
}

type CatalogPricedBillingRuleCreateRes struct {
	BillingRuleNo string `json:"billingRuleNo" dc:"Billing rule created together with its base price" eg:"bil_0123456789abcdef0123456789abcdef"`
}
