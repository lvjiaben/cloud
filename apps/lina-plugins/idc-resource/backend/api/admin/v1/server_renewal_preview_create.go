package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerRenewalPreviewCreateReq struct {
	g.Meta        `path:"/admin/resource/server-renewals/previews" method:"post" tags:"IDC Server Renewals" summary:"Preview selected server renewals" dc:"Quote each of at most 20 servers selected from the resource-owned list for one shared renewal cycle count. Each item reports independent eligibility without exposing internal entitlement identifiers." permission:"idc:admin:resource:server-batch:safe"`
	RequestKey    string   `json:"requestKey" v:"required|length:8,128" dc:"Client-generated idempotency key reused for confirmation" eg:"renewal-20260828-001"`
	RenewalCycles int      `json:"renewalCycles" v:"required|between:1,120" dc:"One renewal cycle count applied to every selected server" eg:"12"`
	ServerNos     []string `json:"serverNos" v:"required" dc:"One through 20 server numbers selected from this plugin's list" eg:"[\"SVR-0123456789ABCDEF0123456789ABCDEF\"]"`
}

type ServerRenewalPreviewCreateRes struct {
	RequestKey         string                      `json:"requestKey"`
	RenewalCycles      int                         `json:"renewalCycles"`
	ConfirmationDigest string                      `json:"confirmationDigest" dc:"Opaque digest binding the exact selection, cycles, quotes, amounts, expiry, and projection versions"`
	Items              []*ServerRenewalPreviewItem `json:"items"`
}

type ServerRenewalPreviewItem struct {
	ServerNo           string `json:"serverNo"`
	ProductNameZhCN    string `json:"productNameZhCN"`
	ProductNameEnUS    string `json:"productNameEnUS"`
	Eligible           bool   `json:"eligible"`
	FailureCode        string `json:"failureCode" dc:"Stable safe failure code; empty when eligible"`
	CurrencyCode       string `json:"currencyCode" eg:"CNY"`
	TotalAmountMinor   string `json:"totalAmountMinor" dc:"Core-owned quoted total in currency minor units as a decimal integer string" eg:"129900"`
	OriginalExpiresAt  *int64 `json:"originalExpiresAt" dc:"Current entitlement expiry as Unix milliseconds"`
	ProjectedExpiresAt *int64 `json:"projectedExpiresAt" dc:"Projected expiry after renewal as Unix milliseconds"`
	QuoteExpiresAt     *int64 `json:"quoteExpiresAt" dc:"Quote expiry as Unix milliseconds"`
}
