// This file defines complete explicit-currency price-schedule creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogPriceTierInput struct {
	MinimumQuantity    int64   `json:"minimumQuantity" v:"min:0" dc:"Inclusive tier lower boundary" eg:"0"`
	MaximumQuantity    *int64  `json:"maximumQuantity" dc:"Exclusive upper boundary; null only on the last tier" eg:"10"`
	AmountMinor        string  `json:"amountMinor" v:"required|regex:^\\d{1,19}$" dc:"Exact new-purchase amount in decimal minor units" eg:"9900"`
	SetupFeeMinor      string  `json:"setupFeeMinor" v:"required|regex:^\\d{1,19}$" dc:"Exact one-time setup fee in decimal minor units; first tier only" eg:"0"`
	RenewalAmountMinor *string `json:"renewalAmountMinor" v:"regex:^\\d{1,19}$" dc:"Exact renewal amount in decimal minor units when required" eg:"10900"`
}

type CatalogPriceScheduleCreateReq struct {
	g.Meta                        `path:"/admin/catalog/product-versions/{versionNo}/billing-rules/{billingRuleNo}/price-schedules" method:"post" tags:"IDC Catalog Management" summary:"Create catalog price schedule" dc:"Atomically create one complete base or explicit configuration-dimension price schedule in one currency." permission:"idc:admin:catalog:price:write"`
	VersionNo                     string                  `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable editable draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	BillingRuleNo                 string                  `json:"billingRuleNo" in:"path" v:"required|max-length:40" dc:"Stable version-owned billing-rule business number" eg:"bil_0123456789abcdef0123456789abcdef"`
	ConfigurationPriceDimensionNo string                  `json:"configurationPriceDimensionNo" v:"max-length:40" dc:"Optional merchant-created configuration price dimension; empty creates the base schedule" eg:""`
	CurrencyCode                  string                  `json:"currencyCode" v:"required|length:3,3" dc:"Explicit uppercase ISO-style three-letter currency code without fallback" eg:"CNY"`
	Tiers                         []CatalogPriceTierInput `json:"tiers" v:"required" dc:"Complete contiguous schedule starting at zero and ending without an upper boundary; collection bounds are enforced by the catalog service"`
	Status                        string                  `json:"status" v:"required|in:active,inactive" dc:"Draft price-schedule status" eg:"active"`
}

type CatalogPriceScheduleTierRes struct {
	PriceNo            string  `json:"priceNo" dc:"Stable price-row business number" eg:"prc_0123456789abcdef0123456789abcdef"`
	MinimumQuantity    int64   `json:"minimumQuantity" dc:"Inclusive tier lower boundary" eg:"0"`
	MaximumQuantity    *int64  `json:"maximumQuantity" dc:"Exclusive upper boundary; null on last tier" eg:"10"`
	AmountMinor        string  `json:"amountMinor" dc:"Exact new-purchase amount in decimal minor units" eg:"9900"`
	SetupFeeMinor      string  `json:"setupFeeMinor" dc:"Exact one-time setup fee in decimal minor units" eg:"0"`
	RenewalAmountMinor *string `json:"renewalAmountMinor" dc:"Exact renewal amount in decimal minor units" eg:"10900"`
}

type CatalogPriceScheduleCreateRes struct {
	VersionNo                     string                         `json:"versionNo" dc:"Owning version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	BillingRuleNo                 string                         `json:"billingRuleNo" dc:"Owning billing-rule business number" eg:"bil_0123456789abcdef0123456789abcdef"`
	ConfigurationPriceDimensionNo string                         `json:"configurationPriceDimensionNo" dc:"Priced configuration dimension number or empty for base price" eg:""`
	CurrencyCode                  string                         `json:"currencyCode" dc:"Explicit currency code" eg:"CNY"`
	Tiers                         []*CatalogPriceScheduleTierRes `json:"tiers" dc:"Committed complete price tiers"`
	Status                        string                         `json:"status" dc:"Stable schedule status" eg:"active"`
}
