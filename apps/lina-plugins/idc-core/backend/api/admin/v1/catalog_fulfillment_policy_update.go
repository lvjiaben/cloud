// This file defines the merchant-facing product-version fulfillment choice.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogUpstreamProductSelection struct {
	RequestKey    string               `json:"requestKey" v:"required|length:16,128" dc:"Idempotency key reused only for an unchanged upstream selection and pricing retry" eg:"upstream-product-0123456789"`
	Reference     string               `json:"reference" v:"required|max-length:256" dc:"Explicit selected upstream product reference" eg:"101"`
	CategoryNo    string               `json:"categoryNo" v:"required|length:36,36" dc:"Current local sales category number" eg:"ctg_0123456789abcdef0123456789abcdef"`
	PricingMode   string               `json:"pricingMode" v:"required|in:markup,manual" dc:"Local price policy: markup percentage or manual final prices" eg:"markup"`
	MarkupPercent string               `json:"markupPercent" v:"max-length:128" dc:"Nonnegative percentage uplift; 10 means cost plus ten percent, empty for manual prices" eg:"10"`
	Prices        []CatalogImportPrice `json:"prices" dc:"At most 120 exact final schedules, required only for manual pricing" eg:"[]"`
}

type CatalogFulfillmentPolicyUpdateReq struct {
	UpstreamProduct     *CatalogUpstreamProductSelection `json:"upstreamProduct,omitempty" dc:"Optional explicit upstream product replacement with immutable source configuration and local pricing; omitted keeps the current product" eg:"{}"`
	g.Meta              `path:"/admin/catalog/product-versions/{versionNo}/fulfillment-policy" method:"put" tags:"IDC Catalog Management" summary:"Update product-version fulfillment mode" dc:"Choose one of the three business fulfillment modes for a draft product version. Plugin execution details are derived by the server from the exact frozen release contract." permission:"idc:admin:catalog:fulfillment-binding:write"`
	VersionNo           string  `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Draft product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	StrategyCode        string  `json:"strategyCode" v:"required|in:manual,approval_then_plugin,payment_confirmed_plugin" dc:"Merchant fulfillment choice: fully manual, approval then plugin, or payment confirmed then plugin" eg:"manual"`
	InterfaceInstanceNo *string `json:"interfaceInstanceNo" v:"max-length:64" dc:"Optional interface selection saved atomically with the fulfillment mode; omitted retains the binding, empty removes it, otherwise an authorized local interface number is required" eg:"pfi_0123456789abcdef0123456789abcdef"`
}

type CatalogFulfillmentPolicyUpdateRes struct {
	Policy *CatalogFulfillmentPolicySummary `json:"policy" dc:"Safe merchant-facing fulfillment policy summary" eg:"{}"`
}
