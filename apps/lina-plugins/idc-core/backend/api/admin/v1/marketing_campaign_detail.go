// This file defines the bounded marketing campaign detail contract.

package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCampaignDetailReq struct {
	g.Meta     `path:"/admin/marketing/campaigns/{campaignNo}" method:"get" tags:"IDC Marketing Management" summary:"Get marketing campaign detail" dc:"Return one bounded campaign definition with explicit product scopes and currency-safe benefit rules." permission:"idc:admin:marketing:campaign:query"`
	CampaignNo string `json:"campaignNo" v:"required|max-length:128" dc:"Stable marketing campaign business number" eg:"campaign-summer-2026"`
}

type MarketingCampaignDetailRes struct {
	Campaign    *MarketingCampaignManagementItem `json:"campaign" dc:"Campaign identity, lifecycle, quota, and rule counts" eg:"{}"`
	Description *MarketingBilingualDescription   `json:"description" dc:"Bilingual campaign description" eg:"{}"`
	StackGroup  string                           `json:"stackGroup" dc:"Campaign stacking group code" eg:"seasonal"`
	Version     int64                            `json:"version" dc:"Optimistic campaign definition version" eg:"3"`
	Scopes      []*MarketingCampaignScope        `json:"scopes" dc:"At most 512 explicit product-scope rules" eg:"[]"`
	Benefits    []*MarketingCampaignBenefit      `json:"benefits" dc:"At most 512 typed benefit rules; every monetary value carries an explicit currency" eg:"[]"`
}
