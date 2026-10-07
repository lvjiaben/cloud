package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelWeChatOfficialConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/wechat-official/service-account-configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure WeChat service account" dc:"Stores one installation-owned verified WeChat service account. Formal delivery remains fail-closed until callback verification is configured; an authorized administrator may still perform one-time real tests." permission:"idc:admin:notification:channel:manage"`
	AppID           string `json:"appId" v:"required|max-length:64" dc:"Verified service account AppID" eg:"wx1234567890abcdef"`
	AppSecret       string `json:"appSecret" v:"required|max-length:256" dc:"Service account AppSecret written only to the authoritative secret service" eg:"secret"`
	CallbackToken   string `json:"callbackToken" v:"required|min-length:3|max-length:64" dc:"Official Account callback verification token written only to the authoritative secret service" eg:"secret"`
	EncodingAESKey  string `json:"encodingAesKey" v:"required|length:43" dc:"Official Account callback EncodingAESKey written only to the authoritative secret service" eg:"abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"wechat-config-20260815-001"`
}

type ChannelWeChatOfficialConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"WeChat Official Account channel projection after configuration" eg:"{}"`
}
