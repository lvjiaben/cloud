// This file exposes simple cost settings embedded in catalog product-version management.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductVersionCostItem struct {
	CostNo         string `json:"costNo" dc:"Stable cost setting reference" eg:"cst_0123456789abcdef0123456789abcdef"`
	CurrencyCode   string `json:"currencyCode" dc:"Uppercase currency code" eg:"CNY"`
	PeriodUnit     string `json:"periodUnit" dc:"Billing period unit" eg:"month"`
	PeriodCount    int    `json:"periodCount" dc:"Number of billing period units" eg:"1"`
	UnitCostMinor  string `json:"unitCostMinor" dc:"Unit cost in decimal minor currency units" eg:"1800"`
	EffectiveFrom  int64  `json:"effectiveFrom" dc:"Effective start time in Unix milliseconds" eg:"1767225600000"`
	EffectiveUntil *int64 `json:"effectiveUntil" dc:"Optional effective end time in Unix milliseconds" eg:"1798761600000"`
	Version        int64  `json:"version" dc:"Concurrency version" eg:"1"`
}

type CatalogProductVersionCostListReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/costs" method:"get" tags:"IDC Catalog Management" summary:"List catalog product version costs" dc:"List simple cost settings for one product version." permission:"idc:admin:catalog:cost:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable product version reference" eg:"ver_0123456789abcdef0123456789abcdef"`
}

type CatalogProductVersionCostListRes struct {
	Costs []*CatalogProductVersionCostItem `json:"costs" dc:"Bounded product-version cost settings" eg:"[]"`
}

type CatalogProductVersionCostCreateReq struct {
	g.Meta         `path:"/admin/catalog/product-versions/{versionNo}/costs" method:"post" tags:"IDC Catalog Management" summary:"Create catalog product version cost" dc:"Add a simple cost setting to one product version." permission:"idc:admin:catalog:cost:write"`
	VersionNo      string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable product version reference" eg:"ver_0123456789abcdef0123456789abcdef"`
	CurrencyCode   string `json:"currencyCode" v:"required|length:3,3" dc:"Uppercase currency code" eg:"CNY"`
	PeriodUnit     string `json:"periodUnit" v:"required|in:once,hour,day,month,year" dc:"Billing period unit" eg:"month"`
	PeriodCount    int    `json:"periodCount" v:"required|min:1|max:87600" dc:"Number of billing period units" eg:"1"`
	UnitCostMinor  string `json:"unitCostMinor" v:"required|regex:^[0-9]{1,19}$" dc:"Unit cost in decimal minor currency units" eg:"1800"`
	EffectiveFrom  *int64 `json:"effectiveFrom" dc:"Optional effective start time in Unix milliseconds" eg:"1767225600000"`
	EffectiveUntil *int64 `json:"effectiveUntil" dc:"Optional effective end time in Unix milliseconds" eg:"1798761600000"`
}

type CatalogProductVersionCostCreateRes struct {
	Cost *CatalogProductVersionCostItem `json:"cost" dc:"Created product-version cost setting" eg:"{}"`
}

type CatalogProductVersionCostUpdateReq struct {
	g.Meta          `path:"/admin/catalog/product-versions/{versionNo}/costs/{costNo}" method:"put" tags:"IDC Catalog Management" summary:"Update catalog product version cost" dc:"Update one simple product-version cost setting." permission:"idc:admin:catalog:cost:write"`
	VersionNo       string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable product version reference" eg:"ver_0123456789abcdef0123456789abcdef"`
	CostNo          string `json:"costNo" in:"path" v:"required|max-length:40" dc:"Stable cost setting reference" eg:"cst_0123456789abcdef0123456789abcdef"`
	CurrencyCode    string `json:"currencyCode" v:"required|length:3,3" dc:"Uppercase currency code" eg:"CNY"`
	PeriodUnit      string `json:"periodUnit" v:"required|in:once,hour,day,month,year" dc:"Billing period unit" eg:"month"`
	PeriodCount     int    `json:"periodCount" v:"required|min:1|max:87600" dc:"Number of billing period units" eg:"1"`
	UnitCostMinor   string `json:"unitCostMinor" v:"required|regex:^[0-9]{1,19}$" dc:"Unit cost in decimal minor currency units" eg:"1800"`
	EffectiveFrom   *int64 `json:"effectiveFrom" dc:"Optional effective start time in Unix milliseconds" eg:"1767225600000"`
	EffectiveUntil  *int64 `json:"effectiveUntil" dc:"Optional effective end time in Unix milliseconds" eg:"1798761600000"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected concurrency version" eg:"1"`
}

type CatalogProductVersionCostUpdateRes struct {
	Cost *CatalogProductVersionCostItem `json:"cost" dc:"Updated product-version cost setting" eg:"{}"`
}
