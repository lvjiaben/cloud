package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCampaignNewcomerUpdateReq struct {
	g.Meta          `path:"/admin/marketing/campaigns/{campaignNo}/newcomer-setting" method:"put" tags:"IDC Marketing Management" summary:"Update newcomer campaign setting" dc:"Enable or disable the one-claim-per-verified-subject rule with optimistic locking and an idempotent request number." permission:"idc:admin:marketing:campaign:newcomer:update"`
	CampaignNo      string `json:"campaignNo" in:"path" v:"required|length:3,64" dc:"Stable campaign business number" eg:"campaign-summer-2026"`
	RequestNo       string `json:"requestNo" v:"required|length:8,64" dc:"Client generated idempotency request number" eg:"request-newcomer-2026-001"`
	Enabled         bool   `json:"enabled" dc:"Whether the campaign is newcomer exclusive" eg:"true"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Current campaign version shown to the operator" eg:"3"`
}

type MarketingCampaignNewcomerUpdateRes struct {
	CampaignNo string `json:"campaignNo" dc:"Stable campaign business number" eg:"campaign-summer-2026"`
	Enabled    bool   `json:"enabled" dc:"Current newcomer-exclusive setting" eg:"true"`
	Version    int64  `json:"version" dc:"Updated campaign version" eg:"4"`
	UpdatedAt  int64  `json:"updatedAt" dc:"Update time in Unix milliseconds" eg:"1767301200000"`
}
