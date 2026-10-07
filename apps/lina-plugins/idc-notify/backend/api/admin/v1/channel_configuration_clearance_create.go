package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelConfigurationClearanceCreateReq struct {
	g.Meta          `path:"/admin/notifications/channels/{channelCode}/configuration-clearance" method:"post" tags:"IDC Notification Channels" summary:"Clear notification channel configuration" dc:"Revokes a disabled external channel credential and clears its merchant configuration after explicit confirmation and safety checks." permission:"idc:admin:notification:channel:manage"`
	ChannelCode     string `json:"channelCode" in:"path" v:"required|max-length:32" dc:"Notification channel code to clear" eg:"email"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version for optimistic concurrency" eg:"3"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key for this clearance action" eg:"channel-clear-20260815-001"`
	Confirmed       bool   `json:"confirmed" v:"required" dc:"Explicit confirmation that the administrator intends to revoke and clear the configuration" eg:"true"`
}
type ChannelConfigurationClearanceCreateRes struct {
	Item *ChannelItem `json:"item" dc:"Channel projection after the configuration is cleared" eg:"{}"`
}
