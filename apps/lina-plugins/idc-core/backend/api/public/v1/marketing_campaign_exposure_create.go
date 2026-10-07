// This file defines privacy-safe public campaign exposure creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCampaignExposureCreateReq struct {
	g.Meta       `path:"/public/catalog/products/{publicSlug}/campaign-exposures" method:"post" tags:"IDC Public Marketing" summary:"Record a public campaign exposure" dc:"Hash the browser-local random device identifier in memory and record at most one exposure per device, campaign, and currency in each rolling thirty-minute window. Raw device identifiers are never persisted."`
	PublicSlug   string `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public product slug" eg:"starter-cloud"`
	CampaignNo   string `json:"campaignNo" v:"required|length:1,40" dc:"Campaign number returned by the matching public campaign list" eg:"mcp_01JABCDEF0123456789ABCDEFG"`
	CurrencyCode string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Visitor-selected uppercase ISO-style currency code" eg:"CNY"`
	DeviceID     string `json:"deviceId" v:"required|length:16,128" dc:"Browser-generated random identifier used only for purpose-separated hashing" eg:"8fbd7527-09f4-4d54-a9af-c3b542ad3f57"`
}

type MarketingCampaignExposureCreateRes struct {
	Recorded bool `json:"recorded" dc:"Whether this call created the exposure fact for the current rolling window" eg:"true"`
}
