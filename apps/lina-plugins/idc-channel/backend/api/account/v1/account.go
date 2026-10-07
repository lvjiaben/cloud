// Package v1 defines the customer-owned downstream channel token HTTP contract.
package v1

import "github.com/gogf/gf/v2/frame/g"

type TokenSummary struct {
	Id          int64    `json:"id" dc:"Opaque token record identifier used only for revocation" eg:"42"`
	Scopes      []string `json:"scopes" dc:"Approved operations granted to this token" eg:"catalog.read"`
	SourceCIDRs []string `json:"sourceCidrs" dc:"Optional source network restrictions in CIDR notation" eg:"192.0.2.0/24"`
	ExpiresAt   *int64   `json:"expiresAt" dc:"Optional token expiry as a Unix timestamp in milliseconds" eg:"1798761600000"`
	RevokedAt   *int64   `json:"revokedAt" dc:"Revocation time as a Unix timestamp in milliseconds when revoked" eg:"1788166800000"`
	CreatedAt   int64    `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1788163200000"`
}
type ListTokensReq struct {
	g.Meta `path:"/account/channel/tokens" method:"get" tags:"Channel Account" summary:"List current approved channel tokens" dc:"List bounded token metadata for the current approved and identity-verified enterprise channel. Token secrets are never returned."`
}
type ListTokensRes struct {
	List []*TokenSummary `json:"list" dc:"Current channel token metadata, bounded by the service limit" eg:"[]"`
}
type CreateTokenReq struct {
	g.Meta      `path:"/account/channel/tokens" method:"post" tags:"Channel Account" summary:"Create one current approved channel token" dc:"Create a channel token for the current approved and identity-verified enterprise channel. The secret is returned exactly once."`
	Scopes      []string `json:"scopes" dc:"One or more approved channel operations" eg:"catalog.read"`
	SourceCIDRs []string `json:"sourceCidrs" dc:"Optional source network restrictions in CIDR notation" eg:"192.0.2.0/24"`
	ExpiresAt   *int64   `json:"expiresAt" dc:"Optional future expiry as a Unix timestamp in milliseconds" eg:"1798761600000"`
}
type CreateTokenRes struct {
	Id        int64  `json:"id" dc:"Opaque token record identifier used only for revocation" eg:"42"`
	Token     string `json:"token" dc:"One-time token secret that is never returned again" eg:"chn_live_example"`
	ExpiresAt *int64 `json:"expiresAt" dc:"Optional token expiry as a Unix timestamp in milliseconds" eg:"1798761600000"`
}
type RevokeTokenReq struct {
	g.Meta `path:"/account/channel/tokens/{id}" method:"delete" tags:"Channel Account" summary:"Revoke one current approved channel token" dc:"Irreversibly revoke one token owned by the current approved enterprise channel."`
	Id     int64 `json:"id" v:"required" dc:"Opaque token record identifier from the current channel token list" eg:"42"`
}
type RevokeTokenRes struct {
	Id      int64 `json:"id" dc:"Opaque revoked token record identifier" eg:"42"`
	Revoked bool  `json:"revoked" dc:"Whether the token is now revoked" eg:"true"`
}
