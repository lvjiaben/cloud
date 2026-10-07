// This file defines shared anonymous catalog response projections.

package v1

// CatalogCategoryItem is one enabled public filter choice.
type CatalogCategoryItem struct {
	CategoryNo string `json:"categoryNo" dc:"Stable public category number" eg:"category_cloud"`
	NameZhCN   string `json:"nameZhCN" dc:"Simplified Chinese category name" eg:"Cloud Server CN"`
	NameEnUS   string `json:"nameEnUS" dc:"English category name" eg:"Cloud servers"`
}

// CatalogProductItem omits internal product/version/pool/provider identities and costs.
type CatalogProductItem struct {
	PublicSlug          string   `json:"publicSlug" dc:"Public browser slug" eg:"starter-cloud"`
	CategoryNo          string   `json:"categoryNo" dc:"Stable public category number" eg:"category_cloud"`
	CategoryNameZhCN    string   `json:"categoryNameZhCN" dc:"Simplified Chinese category name" eg:"Cloud Server CN"`
	CategoryNameEnUS    string   `json:"categoryNameEnUS" dc:"English category name" eg:"Cloud servers"`
	NameZhCN            string   `json:"nameZhCN" dc:"Simplified Chinese sales name" eg:"Starter Cloud CN"`
	NameEnUS            string   `json:"nameEnUS" dc:"English sales name" eg:"Starter cloud server"`
	SummaryZhCN         string   `json:"summaryZhCN" dc:"Simplified Chinese sales summary"`
	SummaryEnUS         string   `json:"summaryEnUS" dc:"English sales summary"`
	Currencies          []string `json:"currencies" dc:"Explicit complete pricing currencies" eg:"CNY"`
	Availability        string   `json:"availability" dc:"Authoritative simple inventory availability: available or unavailable" eg:"available"`
	PricingPresentation string   `json:"pricingPresentation" dc:"Signals that displayed base prices are directly comparable" eg:"base_price"`
}
