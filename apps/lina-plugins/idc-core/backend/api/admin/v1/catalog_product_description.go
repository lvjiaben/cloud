// This file defines metadata-only product updates with optimistic concurrency.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductDescriptionUpdateReq struct {
	g.Meta      `path:"/admin/catalog/products/{productNo}/description" method:"put" tags:"IDC Catalog Management" summary:"Update catalog product sales copy" dc:"Update bilingual names and summaries with exact metadata permission and optimistic concurrency; URL, price, configuration and contract remain unchanged." permission:"idc:admin:catalog:product:update"`
	DirectoryNo *string `json:"directoryNo" dc:"Single product-type member navigation reference; empty keeps services in all services; omitted update preserves the current assignment" eg:"pdr_example"`
	ProductNo   string  `json:"productNo" v:"required|max-length:128" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	Version     int64   `json:"version" v:"required|min:1" dc:"Product description revision last read; stale updates are rejected" eg:"1"`
	NameZhCn    string  `json:"nameZhCn" v:"required|max-length:256" dc:"Simplified Chinese product name" eg:"Cloud server"`
	NameEnUs    string  `json:"nameEnUs" v:"required|max-length:256" dc:"English product name" eg:"Cloud server"`
	SummaryZhCn string  `json:"summaryZhCn" v:"required|max-length:1024" dc:"Simplified Chinese sales summary" eg:"Managed cloud service"`
	SummaryEnUs string  `json:"summaryEnUs" v:"required|max-length:1024" dc:"English sales summary" eg:"Managed cloud service"`
}

type CatalogProductDescriptionUpdateRes struct {
	Version int64 `json:"version" dc:"New product description revision" eg:"2"`
}
