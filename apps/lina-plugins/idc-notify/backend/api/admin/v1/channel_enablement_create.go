package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelEnablementCreateReq struct {
	g.Meta          `path:"/admin/notifications/channels/{channelCode}/enablement" method:"post" tags:"IDC Notification Channels" summary:"Enable notification channel" dc:"Enables a configured notification channel after a recent-login and optimistic-version check." permission:"idc:admin:notification:channel:manage"`
	ChannelCode     string `json:"channelCode" in:"path" v:"required|max-length:32" dc:"Notification channel code to enable" eg:"email"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version for optimistic concurrency" eg:"3"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key for this enablement action" eg:"channel-enable-20260815-001"`
	Confirmed       bool   `json:"confirmed" v:"required" dc:"Explicit confirmation that the administrator intends to enable the channel" eg:"true"`
}
type ChannelEnablementCreateRes struct {
	Item *ChannelItem `json:"item" dc:"Channel projection after enablement" eg:"{}"`
}
