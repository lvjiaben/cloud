// This file declares concurrency-protected admission changes without rewriting historical money.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CurrencyUpdateReq struct {
	g.Meta          `path:"/admin/currencies/{currencyCode}" method:"put" tags:"IDC Currency" summary:"Update merchant currency" dc:"Enable or disable a currency and optionally make it the default; rejects stale versions and disabling the current default, preserving historical prices and balances." permission:"idc:admin:currency:write"`
	CurrencyCode    string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Existing ISO 4217 denomination" eg:"USD"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last displayed currency version" eg:"1"`
	Enabled         bool   `json:"enabled" dc:"Whether new quotations may use this denomination" eg:"true"`
	MakeDefault     bool   `json:"makeDefault" dc:"Make this enabled denomination the default; false keeps the current default" eg:"false"`
}
type CurrencyUpdateRes struct{ CurrencyItem }
