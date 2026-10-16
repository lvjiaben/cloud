// This file exposes bounded, display-safe product and staff candidates.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementProductsReq struct {
	g.Meta `path:"/admin/sales-management/product-candidates" method:"get" tags:"IDC Sales Management" summary:"List sales product candidates" dc:"Search published products and their explicitly priced billing periods without exposing internal configuration." permission:"idc:admin:sales-channel:overview:view"`
	SalesManagementPageQuery
}
type SalesManagementProductTerm struct {
	CurrencyCode   string `json:"currencyCode" dc:"Explicitly configured currency" eg:"CNY"`
	ChargeTypeCode string `json:"chargeTypeCode" dc:"Catalog charge type" eg:"recurring"`
	PeriodUnitCode string `json:"periodUnitCode" dc:"Catalog period unit" eg:"month"`
	PeriodCount    int    `json:"periodCount" dc:"Number of units per billing period" eg:"1"`
}
type SalesManagementProduct struct {
	ProductNo   string                       `json:"productNo" dc:"Stable product reference" eg:"PRD_example"`
	Name        string                       `json:"name" dc:"Chinese display name" eg:"Cloud server"`
	EnglishName string                       `json:"englishName" dc:"English display name" eg:"Cloud server"`
	Terms       []SalesManagementProductTerm `json:"terms" dc:"Available currency and billing combinations" eg:"[]"`
}
type SalesManagementProductsRes struct {
	Currencies []string                 `json:"currencies" dc:"Configured merchant and product currencies" eg:"[]"`
	Items      []SalesManagementProduct `json:"items" dc:"Current page records" eg:"[]"`
	Total      int                      `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementStaffReq struct {
	g.Meta `path:"/admin/sales-management/staff-candidates" method:"get" tags:"IDC Sales Management" summary:"List sales staff candidates" dc:"Search enabled visible directory users; only active staff identities are selectable. Pagination and totals belong to the governed directory." permission:"idc:admin:sales-channel:assignment:manage"`
	SalesManagementPageQuery
}
type SalesManagementStaff struct {
	UserID     int    `json:"userId" dc:"Visible staff user identifier" eg:"1"`
	Name       string `json:"name" dc:"Staff display name" eg:"Alice"`
	Selectable bool   `json:"selectable" dc:"Whether this visible user is an active staff identity eligible for sales assignment" eg:"true"`
}
type SalesManagementStaffRes struct {
	Items []SalesManagementStaff `json:"items" dc:"Current page records" eg:"[]"`
	Total int                    `json:"total" dc:"Total matching directory records before staff classification" eg:"0"`
}

// SalesManagementPriceProductsReq scopes catalog choices to the price operator's capability.
type SalesManagementPriceProductsReq struct {
	g.Meta `path:"/admin/sales-management/price-product-candidates" method:"get" tags:"IDC Sales Management" summary:"List price product candidates" dc:"Search published products or resolve one exact product and its currency-period choices." permission:"idc:admin:sales-channel:pricing:manage"`
	SalesManagementPageQuery
	ProductNo string `json:"productNo" v:"max-length:128" dc:"Exact product reference filter" eg:"PRD_example"`
}
type SalesManagementPriceProductsRes struct {
	Currencies []string                 `json:"currencies" dc:"Configured merchant and product currencies" eg:"[]"`
	Items      []SalesManagementProduct `json:"items" dc:"Current page records" eg:"[]"`
	Total      int                      `json:"total" dc:"Total matching records" eg:"0"`
}

// SalesManagementPriceLevelsReq returns display-safe tier choices for pricing.
type SalesManagementPriceLevelsReq struct {
	g.Meta `path:"/admin/sales-management/price-level-candidates" method:"get" tags:"IDC Sales Management" summary:"List price tier candidates" dc:"Return a bounded page of named agent tiers for product pricing." permission:"idc:admin:sales-channel:pricing:manage"`
	SalesManagementPageQuery
}
type SalesManagementPriceLevelsRes struct {
	Items []SalesManagementLevel `json:"items" dc:"Current page records" eg:"[]"`
	Total int                    `json:"total" dc:"Total matching records" eg:"0"`
}
