// This file declares version-protected removal of currency configuration while preserving money history.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CurrencyDeleteReq struct {
	g.Meta          `path:"/admin/currencies/{currencyCode}" method:"delete" tags:"IDC Currency" summary:"Delete merchant currency" dc:"Remove currency configuration and reject new quotes; preserve all product prices, orders and balances. Switch the default first. Stale versions are rejected." permission:"idc:admin:currency:write"`
	CurrencyCode    string `json:"currencyCode" in:"path" v:"required|regex:^[A-Z]{3}$" dc:"Existing ISO 4217 currency code" eg:"USD"`
	ExpectedVersion int64  `json:"expectedVersion" in:"query" v:"required|min:1" dc:"Last displayed currency version" eg:"1"`
}
type CurrencyDeleteRes struct{}
