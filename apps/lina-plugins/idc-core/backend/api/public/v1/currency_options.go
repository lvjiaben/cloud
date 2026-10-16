// This file publishes non-sensitive denomination choices; it does not publish balances or exchange rates.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CurrencyOptionsReq struct {
	g.Meta `path:"/currencies" method:"get" tags:"IDC Currency" summary:"Get enabled currencies" dc:"Return at most 300 enabled merchant denominations, default first; product prices and payment-channel support remain independently validated."`
}
type CurrencyOptionsRes struct {
	Currencies []string `json:"currencies" dc:"Enabled ISO currency codes; default first" eg:"[\"CNY\",\"USD\"]"`
}
