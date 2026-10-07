package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCouponItem struct {
	AssetNo         string `json:"assetNo" dc:"Stable promotion asset number"`
	CampaignNo      string `json:"campaignNo" dc:"Owning campaign number"`
	NameZhCn        string `json:"nameZhCn" dc:"Simplified Chinese campaign name"`
	NameEnUs        string `json:"nameEnUs" dc:"English campaign name"`
	AssetType       string `json:"assetType" dc:"Coupon or voucher"`
	CurrencyCode    string `json:"currencyCode" dc:"Explicit benefit currency"`
	FaceAmountMinor *int64 `json:"faceAmountMinor" dc:"Fixed benefit in minor units, or null"`
	RateBasisPoints *int   `json:"rateBasisPoints" dc:"Percentage benefit in basis points, or null"`
	UsageLimit      int    `json:"usageLimit" dc:"Maximum uses"`
	ConsumedUses    int    `json:"consumedUses" dc:"Already consumed uses"`
	Status          string `json:"status" dc:"Current asset status"`
	ValidFrom       int64  `json:"validFrom" dc:"Validity start in Unix milliseconds"`
	ValidUntil      int64  `json:"validUntil" dc:"Validity end in Unix milliseconds"`
	ClaimedAt       *int64 `json:"claimedAt" dc:"Claim time in Unix milliseconds"`
}

type MarketingCouponListReq struct {
	g.Meta `path:"/account/marketing/coupons" method:"get" tags:"IDC Customer Marketing" summary:"List my promotion assets" dc:"Return at most one hundred coupons and vouchers owned by the current customer."`
}
type MarketingCouponListRes struct {
	Items []*MarketingCouponItem `json:"items" dc:"Current customer's bounded promotion assets"`
}

type MarketingCouponClaimReq struct {
	g.Meta   `path:"/account/marketing/coupons/claims" method:"post" tags:"IDC Customer Marketing" summary:"Claim a promotion code" dc:"Claim one case-insensitive promotion code while applying privacy-safe device and IP abuse limits."`
	Code     string `json:"code" v:"required|length:8,24" dc:"Eight to twenty-four letter and digit promotion code"`
	DeviceID string `json:"deviceId" v:"required|length:16,128" dc:"Browser-local random device identifier; raw value is never persisted"`
}
type MarketingCouponClaimRes struct {
	Item *MarketingCouponItem `json:"item" dc:"Claimed promotion asset"`
}
