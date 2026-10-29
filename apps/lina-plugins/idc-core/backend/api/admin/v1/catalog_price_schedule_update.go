// This file updates a draft schedule using an existing row as its concurrency fence.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogPriceScheduleUpdateReq struct {
	g.Meta                        `path:"/admin/catalog/product-versions/{versionNo}/billing-rules/{billingRuleNo}/price-schedules/{priceNo}" method:"put" tags:"IDC Catalog Management" summary:"Update draft price schedule" dc:"Replace one complete draft schedule atomically; the existing price row rejects stale edits and published versions are immutable." permission:"idc:admin:catalog:price:write"`
	VersionNo                     string                  `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Editable draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	BillingRuleNo                 string                  `json:"billingRuleNo" in:"path" v:"required|max-length:40" dc:"Version-owned billing-rule business number" eg:"bil_0123456789abcdef0123456789abcdef"`
	PriceNo                       string                  `json:"priceNo" in:"path" v:"required|max-length:40" dc:"Existing price row identifying the complete schedule and stale-edit fence" eg:"prc_0123456789abcdef0123456789abcdef"`
	ConfigurationPriceDimensionNo string                  `json:"configurationPriceDimensionNo" v:"max-length:40" dc:"Existing configuration price dimension; empty for base prices" eg:""`
	CurrencyCode                  string                  `json:"currencyCode" v:"required|length:3,3" dc:"Existing explicit uppercase currency code" eg:"CNY"`
	Tiers                         []CatalogPriceTierInput `json:"tiers" v:"required" dc:"Complete contiguous replacement schedule" eg:"[{\"minimumQuantity\":0,\"amountMinor\":\"1000\",\"setupFeeMinor\":\"0\"}]"`
	Status                        string                  `json:"status" v:"required|in:active,inactive" dc:"Draft schedule status: active or inactive" eg:"active"`
}
type CatalogPriceScheduleUpdateRes = CatalogPriceScheduleCreateRes
