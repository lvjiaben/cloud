// This file declares the permission-gated numbered merchant currency directory.
package v1

import "github.com/gogf/gf/v2/frame/g"

// CurrencyItem is non-sensitive merchant configuration, not a customer balance.
type CurrencyItem struct {
	CurrencyCode string `json:"currencyCode" dc:"ISO 4217 currency code" eg:"CNY"`
	Enabled      bool   `json:"enabled" dc:"Whether this denomination admits new quotes" eg:"true"`
	IsDefault    bool   `json:"isDefault" dc:"Whether this is the merchant default denomination" eg:"true"`
	Version      int64  `json:"version" dc:"Current optimistic concurrency version" eg:"1"`
}
type CurrencyListReq struct {
	g.Meta   `path:"/admin/currencies" method:"get" tags:"IDC Currency" summary:"List merchant currencies" dc:"Return one numbered page of merchant currency admission settings; requires direct staff with supported whole-merchant scope." permission:"idc:admin:currency:read"`
	Page     int `json:"page" v:"min:1|max:1000" dc:"Page number; defaults to one" eg:"1"`
	PageSize int `json:"pageSize" v:"min:1|max:100" dc:"Page size; defaults to twenty" eg:"20"`
}
type CurrencyListRes struct {
	Items []CurrencyItem `json:"items" dc:"Current merchant currency settings" eg:"[]"`
	Total int            `json:"total" dc:"Exact directory size in the same authorized scope" eg:"1"`
}
