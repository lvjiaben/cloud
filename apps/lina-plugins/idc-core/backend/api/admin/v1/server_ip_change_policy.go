package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerIPChangePolicy struct {
	PolicyNo                  string `json:"policyNo" dc:"Stable policy number" eg:"IPC_0123456789abcdef"`
	CurrencyCode              string `json:"currencyCode" dc:"Policy currency" eg:"CNY"`
	Enabled                   bool   `json:"enabled" dc:"Whether customers may request IP changes" eg:"true"`
	FreeChangesPerWindow      int    `json:"freeChangesPerWindow" dc:"Free changes per server in the rolling window" eg:"1"`
	WindowDays                int    `json:"windowDays" dc:"Rolling qualification window in days" eg:"30"`
	MaxChangesPerDay          int    `json:"maxChangesPerDay" dc:"Maximum requests per server per calendar day" eg:"2"`
	AmountMinor               int64  `json:"amountMinor" dc:"Tax-inclusive price after the free allowance" eg:"1000"`
	MaximumSnapshotAgeSeconds int    `json:"maximumSnapshotAgeSeconds" dc:"Maximum accepted resource inventory age" eg:"300"`
	Version                   int64  `json:"version" dc:"Current immutable policy version" eg:"1"`
	PublishedAt               int64  `json:"publishedAt" dc:"Publication time in Unix milliseconds" eg:"1787184000000"`
}

type ServerIPChangePolicyListReq struct {
	g.Meta `path:"/admin/resource/ip-change-policies" method:"get" tags:"IDC Resource Operations" summary:"List server IP-change policies" dc:"List the current bounded per-currency IP-change rules after exact staff authorization." permission:"idc:admin:resource:ip-change-policy:read"`
}

type ServerIPChangePolicyListRes struct {
	Items []*ServerIPChangePolicy `json:"items" dc:"Current per-currency policy versions" eg:"[]"`
}

type ServerIPChangePolicyPublishReq struct {
	g.Meta                    `path:"/admin/resource/ip-change-policies" method:"post" tags:"IDC Resource Operations" summary:"Publish a server IP-change policy" dc:"Append one immutable per-currency qualification, frequency, price, and inventory-freshness rule." permission:"idc:admin:resource:ip-change-policy:write"`
	CurrencyCode              string `json:"currencyCode" v:"required|length:3" dc:"ISO-style uppercase currency code" eg:"CNY"`
	Enabled                   bool   `json:"enabled" dc:"Whether customers may request IP changes" eg:"true"`
	FreeChangesPerWindow      int    `json:"freeChangesPerWindow" v:"min:0|max:100" dc:"Free changes per server in the rolling window" eg:"1"`
	WindowDays                int    `json:"windowDays" v:"required|min:1|max:365" dc:"Rolling window length in days" eg:"30"`
	MaxChangesPerDay          int    `json:"maxChangesPerDay" v:"required|min:1|max:20" dc:"Daily request limit per server" eg:"2"`
	AmountMinor               int64  `json:"amountMinor" v:"min:0|max:1000000000" dc:"Tax-inclusive price after the free allowance" eg:"1000"`
	MaximumSnapshotAgeSeconds int    `json:"maximumSnapshotAgeSeconds" v:"required|min:60|max:86400" dc:"Maximum accepted resource inventory age" eg:"300"`
	ExpectedVersion           int64  `json:"expectedVersion" v:"min:0" dc:"Current version, or zero for the first publication" eg:"0"`
}

type ServerIPChangePolicyPublishRes struct {
	Policy *ServerIPChangePolicy `json:"policy" dc:"Newly published immutable policy version" eg:"{}"`
}
