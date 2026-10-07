// This file defines the merchant-facing product-version fulfillment choice.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogFulfillmentPolicyUpdateReq struct {
	g.Meta       `path:"/admin/catalog/product-versions/{versionNo}/fulfillment-policy" method:"put" tags:"IDC Catalog Management" summary:"Update product-version fulfillment mode" dc:"Choose one of the three business fulfillment modes for a draft product version. Plugin execution details are derived by the server from the exact frozen release contract." permission:"idc:admin:catalog:fulfillment-binding:write"`
	VersionNo    string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Draft product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	StrategyCode string `json:"strategyCode" v:"required|in:manual,approval_then_plugin,payment_confirmed_plugin" dc:"Merchant fulfillment choice: fully manual, approval then plugin, or payment confirmed then plugin" eg:"manual"`
}

type CatalogFulfillmentPolicyUpdateRes struct {
	Policy *CatalogFulfillmentPolicySummary `json:"policy" dc:"Safe merchant-facing fulfillment policy summary" eg:"{}"`
}
