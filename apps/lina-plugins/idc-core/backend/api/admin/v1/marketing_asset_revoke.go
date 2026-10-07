package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingAssetRevokeReq struct {
	g.Meta            `path:"/admin/marketing/assets/{assetNo}/revoke" method:"post" tags:"IDC Marketing Management" summary:"Revoke promotion asset" dc:"Revoke one issued or active unreserved promotion asset with optimistic concurrency." permission:"idc:admin:marketing:asset:revoke"`
	AssetNo           string `json:"assetNo" v:"required|max-length:128" dc:"Promotion asset business number" eg:"asset-summer-001"`
	RequestNo         string `json:"requestNo" v:"required|max-length:128" dc:"Client-stable idempotency request number" eg:"marketing-revoke-001"`
	ExpectedUpdatedAt int64  `json:"expectedUpdatedAt" v:"required|min:1" dc:"Expected asset update time in Unix milliseconds" eg:"1767225600000"`
	ReasonCode        string `json:"reasonCode" v:"required|max-length:64" dc:"Stable operational reason code" eg:"risk_control"`
}

type MarketingAssetRevokeRes struct {
	Mutation *MarketingManagedAssetMutation `json:"mutation" dc:"Audited promotion asset revocation result" eg:"{}"`
}
