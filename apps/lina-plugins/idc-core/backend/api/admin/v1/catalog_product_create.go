// This file defines staff-only manual product-draft creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductCreateReq struct {
	g.Meta                   `path:"/admin/catalog/products" method:"post" tags:"IDC Catalog Management" summary:"Create manual catalog product draft" dc:"Create one product and its first empty manual draft version transactionally." permission:"idc:admin:catalog:product:create"`
	CategoryNo               string `json:"categoryNo" v:"required|length:36,36" dc:"Stable owning category business number" eg:"ctg_0123456789abcdef0123456789abcdef"`
	PublicSlug               string `json:"publicSlug" v:"required|max-length:128" dc:"Lowercase public URL slug" eg:"hong-kong-cloud"`
	NameZhCn                 string `json:"nameZhCn" v:"required|max-length:256" dc:"Simplified Chinese product name" eg:"Hong Kong Cloud zh-CN"`
	NameEnUs                 string `json:"nameEnUs" v:"required|max-length:256" dc:"English product name" eg:"Hong Kong Cloud"`
	SummaryZhCn              string `json:"summaryZhCn" v:"max-length:1024" dc:"Optional Simplified Chinese sales summary" eg:"Low latency zh-CN"`
	SummaryEnUs              string `json:"summaryEnUs" v:"max-length:1024" dc:"Optional English sales summary" eg:"Low-latency cloud servers"`
	SortOrder                int    `json:"sortOrder" v:"between:-1000000,1000000" dc:"Stable product display order" eg:"100"`
	ContractPolicyCode       string `json:"contractPolicyCode" v:"required|in:inherit,none,required" dc:"Product contract choice: inherit enterprise, explicitly none, or required" eg:"inherit"`
	ContractTemplateNo       string `json:"contractTemplateNo" v:"max-length:40" dc:"Required all-customer template number, otherwise empty"`
	InventoryTrackingEnabled bool   `json:"inventoryTrackingEnabled" dc:"Whether this product version limits sales by the configured quantity" eg:"true"`
	SellableQuantity         int64  `json:"sellableQuantity" v:"min:0|max:9223372036854775807" dc:"Maximum sellable quantity when inventory tracking is enabled" eg:"100"`
}

type CatalogProductCreateRes struct {
	ProductNo     string `json:"productNo" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	VersionNo     string `json:"versionNo" dc:"Stable initial draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	VersionNumber int    `json:"versionNumber" dc:"Initial version sequence" eg:"1"`
	CategoryNo    string `json:"categoryNo" dc:"Stable category business number" eg:"ctg_0123456789abcdef0123456789abcdef"`
	Status        string `json:"status" dc:"Product status" eg:"draft"`
	VersionStatus string `json:"versionStatus" dc:"Initial version status" eg:"draft"`
	Source        string `json:"source" dc:"Initial version source" eg:"manual"`
}
