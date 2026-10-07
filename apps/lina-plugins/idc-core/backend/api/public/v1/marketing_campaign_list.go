// This file defines the anonymous product-scoped public campaign list.

package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCampaignListReq struct {
	g.Meta       `path:"/public/catalog/products/{publicSlug}/campaigns" method:"get" tags:"IDC Public Marketing" summary:"List public campaigns for one product" dc:"Return only currently active campaigns with no target segment whose include and exclude scopes match the active published product and whose active benefit matches the selected currency."`
	PublicSlug   string `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public product slug" eg:"starter-cloud"`
	CurrencyCode string `json:"currencyCode" in:"query" v:"required|regex:^[A-Z]{3}$" dc:"Visitor-selected uppercase ISO-style currency code" eg:"CNY"`
}

type PublicMarketingCampaignItem struct {
	CampaignNo      string `json:"campaignNo" dc:"Opaque public campaign number" eg:"mcp_01JABCDEF0123456789ABCDEFG"`
	CampaignType    string `json:"campaignType" dc:"Campaign type: discount, coupon, voucher, points, recharge_bonus, cashback, or customer_price" eg:"discount"`
	NameZhCN        string `json:"nameZhCN" dc:"Simplified Chinese campaign name" eg:"New purchase discount in Simplified Chinese"`
	NameEnUS        string `json:"nameEnUS" dc:"English campaign name" eg:"New purchase discount"`
	DescriptionZhCN string `json:"descriptionZhCN" dc:"Simplified Chinese public campaign description" eg:"Public campaign description in Simplified Chinese"`
	DescriptionEnUS string `json:"descriptionEnUS" dc:"English public campaign description" eg:"Save on a new server purchase"`
	StartsAt        int64  `json:"startsAt" dc:"Campaign start time as a Unix timestamp in milliseconds" eg:"1787328000000"`
	EndsAt          int64  `json:"endsAt" dc:"Campaign end time as a Unix timestamp in milliseconds" eg:"1789920000000"`
}

type MarketingCampaignListRes struct {
	Campaigns []*PublicMarketingCampaignItem `json:"campaigns" dc:"Bounded public campaigns ordered by published priority and campaign number" eg:"[]"`
}
