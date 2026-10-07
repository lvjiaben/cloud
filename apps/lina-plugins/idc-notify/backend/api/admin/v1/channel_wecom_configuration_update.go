package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelWeComConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/wechat-work/custom-app-configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure WeCom custom application" dc:"Stores one installation-owned WeCom custom application. Delivery and test sending remain fail-closed until an authenticated employee self-binding capability is published; administrators cannot enter employee UserIDs." permission:"idc:admin:notification:channel:manage"`
	CorpID          string `json:"corpId" v:"required|max-length:64" dc:"WeCom corporation ID" eg:"ww1234567890abcdef"`
	AgentID         int64  `json:"agentId" v:"min:1" dc:"WeCom custom application agent ID" eg:"1000002"`
	AppSecret       string `json:"appSecret" v:"required|max-length:256" dc:"Custom application secret written only to the authoritative secret service" eg:"secret"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"wecom-config-20260815-001"`
}

type ChannelWeComConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"WeCom channel projection after configuration" eg:"{}"`
}
