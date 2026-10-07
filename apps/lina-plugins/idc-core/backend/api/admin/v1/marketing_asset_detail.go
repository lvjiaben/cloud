package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingAssetDetailReq struct {
	g.Meta  `path:"/admin/marketing/assets/{assetNo}" method:"get" tags:"IDC Marketing Management" summary:"Get promotion asset detail" dc:"Return one coupon or voucher with bounded reservation history." permission:"idc:admin:marketing:asset:query"`
	AssetNo string `json:"assetNo" v:"required|max-length:128" dc:"Stable promotion asset business number" eg:"asset-summer-001"`
}

type MarketingAssetDetailRes struct {
	Asset        *MarketingPromotionAssetItem     `json:"asset" dc:"Promotion asset identity, value, ownership, and lifecycle" eg:"{}"`
	Reservations []*MarketingAssetReservationItem `json:"reservations" dc:"At most one hundred recent reservation records" eg:"[]"`
}
