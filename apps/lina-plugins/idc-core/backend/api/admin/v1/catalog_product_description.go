// This file defines metadata-only product updates with optimistic concurrency.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductDescriptionUpdateReq struct {
	VersionNo       string                       `json:"versionNo,omitempty" v:"max-length:128" dc:"Editable version belonging to this product; required when inventory or login rules are supplied" eg:"ver_0123456789abcdef0123456789abcdef"`
	Inventory       *CatalogBasicInventoryInput  `json:"inventory,omitempty" dc:"Optional changed inventory settings; omitted preserves stock and requires no inventory-write permission" eg:"{}"`
	AddonProductNos *[]string                    `json:"addonProductNos,omitempty" v:"max-length:100" dc:"Optional complete companion product set, at most 100 references; omitted preserves the set, empty clears it" eg:"[]"`
	LoginRules      *CatalogBasicLoginRulesInput `json:"loginRules,omitempty" dc:"Optional changed server login generation rules; preserves customer options and requires independent configuration-write permission" eg:"{}"`
	SortOrder       *int                         `json:"sortOrder" v:"between:-1000000,1000000" dc:"Optional ascending product display order; smaller values appear first, omitted preserves the current order" eg:"10"`
	g.Meta          `path:"/admin/catalog/products/{productNo}/description" method:"put" tags:"IDC Catalog Management" summary:"Update catalog product basic information" dc:"Atomically update bilingual sales copy, category, member navigation and optional changed inventory, companion products and server login rules. Each section retains its own permission and revision check; any failure rolls back the entire request. Prices and historical orders remain unchanged." permission:"idc:admin:catalog:product:update"`
	CategoryNo      *string `json:"categoryNo" v:"max-length:128" dc:"Optional active sales category business number; omitted preserves the current category, empty is rejected; historical orders remain unchanged" eg:"ctg_0123456789abcdef0123456789abcdef"`
	DirectoryNo     *string `json:"directoryNo" dc:"Single required product-type member navigation reference when supplied; empty is rejected, omitted update preserves the current assignment" eg:"pdr_example"`
	ProductNo       string  `json:"productNo" v:"required|max-length:128" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	Version         int64   `json:"version" v:"required|min:1" dc:"Product description revision last read; stale updates are rejected" eg:"1"`
	NameZhCn        string  `json:"nameZhCn" v:"required|max-length:256" dc:"Simplified Chinese product name" eg:"Cloud server"`
	NameEnUs        string  `json:"nameEnUs" v:"required|max-length:256" dc:"English product name" eg:"Cloud server"`
	SummaryZhCn     string  `json:"summaryZhCn" v:"required|max-length:1024" dc:"Simplified Chinese sales summary" eg:"Managed cloud service"`
	SummaryEnUs     string  `json:"summaryEnUs" v:"required|max-length:1024" dc:"English sales summary" eg:"Managed cloud service"`
}

// CatalogBasicInventoryInput retains the independent stock concurrency fence.
type CatalogBasicInventoryInput struct {
	TrackingEnabled  bool  `json:"trackingEnabled" dc:"Whether sales are limited by the available quantity" eg:"true"`
	SellableQuantity int64 `json:"sellableQuantity" v:"min:0" dc:"Nonnegative available stock quantity" eg:"10"`
	ExpectedVersion  int64 `json:"expectedVersion" v:"required|min:1" dc:"Last-read inventory revision; stale stock is rejected" eg:"1"`
}

// CatalogBasicLoginRulesInput only updates server login generation fields.
type CatalogBasicLoginRulesInput struct {
	ExpectedVersion int64                                  `json:"expectedVersion" v:"min:0" dc:"Last-read configuration revision; stale configuration is rejected" eg:"1"`
	Values          []*CatalogStructuredConfigurationValue `json:"values" dc:"Complete server hostname and password generation values and customer-input switches, at most eight fields; other configuration fields are rejected" eg:"[]"`
}

type CatalogProductDescriptionUpdateRes struct {
	Version int64 `json:"version" dc:"New product description revision" eg:"2"`
}
