// This file declares creation of a valid ISO denomination; no exchange rates or prices are generated.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CurrencyCreateReq struct {
	g.Meta       `path:"/admin/currencies" method:"post" tags:"IDC Currency" summary:"Add merchant currency" dc:"Enable one new ISO denomination with transaction audit; existing denominations must be edited instead." permission:"idc:admin:currency:write"`
	CurrencyCode string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"ISO 4217 currency code; XXX is not allowed" eg:"USD"`
}
type CurrencyCreateRes struct{ CurrencyItem }
