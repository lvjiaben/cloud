package v1

import "github.com/gogf/gf/v2/frame/g"

type EventPolicyUpdateReq struct {
	g.Meta          `path:"/admin/notifications/event-policies/{eventCode}/{recipientKind}/{channelCode}" method:"put" tags:"IDC Notification Event Policies" summary:"Update notification event policy" dc:"Changes one exact event, recipient-kind, and channel switch behind an optimistic-version fence." permission:"idc:admin:notification:channel:manage"`
	EventCode       string `json:"eventCode" in:"path" v:"required|max-length:96" dc:"Business notification event code" eg:"support.ticket_created"`
	RecipientKind   string `json:"recipientKind" in:"path" v:"required|in:customer_contact,staff_user,verified_destination" dc:"Authoritative recipient kind" eg:"customer_contact"`
	ChannelCode     string `json:"channelCode" in:"path" v:"required|max-length:32" dc:"Notify-owned channel code" eg:"email"`
	Enabled         bool   `json:"enabled" dc:"Whether this event may use the channel" eg:"true"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected policy version for optimistic concurrency" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"event-policy-20260815-001"`
}

type EventPolicyUpdateRes struct {
	Item *EventPolicyItem `json:"item" dc:"Updated event policy" eg:"{}"`
}
