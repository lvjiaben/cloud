// This file defines customer-scoped purchased-product pagination for staff.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerProductsReq pages order-backed products after staff customer-scope checks.
type CustomerProductsReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/products" method:"get" tags:"IDC Customers" summary:"List customer products" dc:"Page all purchased product types and their current deliveries for one visible customer." permission:"idc:admin:customer:query"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	Page       int    `json:"page" d:"1" v:"between:1,1000000" dc:"Page number" eg:"1"`
	PageSize   int    `json:"pageSize" d:"10" v:"between:1,100" dc:"Page size" eg:"1"`
}

// CustomerProduct exposes display-safe frozen order facts and current lifecycle metadata.
type CustomerProduct struct {
	DeliveryKind    string `json:"deliveryKind" dc:"Delivery kind: server, entitlement or pending; pending products cannot be edited" eg:"server"`
	Number          string `json:"number" dc:"Delivered product number or pending order item number" eg:"SVC_SERVER_0001"`
	OrderNo         string `json:"orderNo" dc:"Source order number" eg:"ORD_0123456789abcdef"`
	ProductNo       string `json:"productNo" dc:"Catalog product number" eg:"PRD_0123456789abcdef"`
	NameZhCn        string `json:"nameZhCn" dc:"Frozen Chinese product name" eg:"Cloud server"`
	NameEnUs        string `json:"nameEnUs" dc:"Frozen English product name" eg:"Cloud server"`
	InformationEnUs string `json:"informationEnUs" dc:"Safe English configuration labels and note" eg:"4 cores / 8 GB"`
	Information     string `json:"information" dc:"Safe Chinese configuration labels and note" eg:"4 cores / 8 GB"`
	AmountMinor     string `json:"amountMinor" dc:"Current product price or frozen purchase price in minor units" eg:"12000"`
	CurrencyCode    string `json:"currencyCode" dc:"ISO currency code" eg:"CNY"`
	ProductType     string `json:"productType" dc:"Product delivery type" eg:"cloud_server"`
	PluginID        string `json:"pluginId" dc:"Installed delivery owner plugin identifier" eg:"idc-resource"`
	PeriodUnit      string `json:"periodUnit" dc:"Billing period unit" eg:"month"`
	PeriodCount     int    `json:"periodCount" dc:"Billing period count" eg:"1"`
	CreatedAt       int64  `json:"createdAt" dc:"Purchase time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	ExpiresAt       *int64 `json:"expiresAt" dc:"Expiry as a Unix timestamp in milliseconds, null when inapplicable" eg:"1787184000000"`
	Status          string `json:"status" dc:"Current delivery or order status" eg:"active"`
}

// CustomerProductsRes contains an exact scoped count and bounded page.
type CustomerProductsRes struct {
	Items []CustomerProduct `json:"items" dc:"Current page of purchased products" eg:"[]"`
	Total int               `json:"total" dc:"Exact customer product count" eg:"1"`
}
