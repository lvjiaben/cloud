package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSMSBaoConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/sms/smsbao-configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure SMSBao" dc:"Selects one installation-owned SMSBao account and the fixed domestic HTTPS API v1 adapter." permission:"idc:admin:notification:channel:manage"`
	SignName        string `json:"signName" v:"required|max-length:64" dc:"Reported SMS signature without brackets" eg:"LinaIDC"`
	ProductID       string `json:"productId" v:"max-length:32" dc:"Optional SMSBao dedicated product ID" eg:"10001"`
	Username        string `json:"username" v:"required|max-length:128" dc:"SMSBao username written only to the authoritative secret service" eg:"merchant"`
	APIKey          string `json:"apiKey" v:"required|max-length:256" dc:"SMSBao API key or MD5 password written only to the authoritative secret service" eg:"secret"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"smsbao-config-20260816-001"`
}

type ChannelSMSBaoConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"SMS channel projection after configuration" eg:"{}"`
}
