package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelDisablementCreateReq struct {
	g.Meta          `path:"/admin/notifications/channels/{channelCode}/disablement" method:"post" tags:"IDC Notification Channels" summary:"Disable notification channel" dc:"Disables a configured channel and cancels its not-yet-sent queued deliveries after explicit confirmation." permission:"idc:admin:notification:channel:manage"`
	ChannelCode     string `json:"channelCode" in:"path" v:"required|max-length:32" dc:"Notification channel code to disable" eg:"email"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version for optimistic concurrency" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key for this disablement action" eg:"channel-disable-20260815-001"`
	Confirmed       bool   `json:"confirmed" v:"required" dc:"Explicit confirmation that the administrator intends to disable the channel" eg:"true"`
}
type ChannelDisablementCreateRes struct {
	Item *ChannelItem `json:"item" dc:"Channel projection after disablement" eg:"{}"`
}
