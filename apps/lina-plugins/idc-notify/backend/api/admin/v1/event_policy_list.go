package v1

import "github.com/gogf/gf/v2/frame/g"

type EventPolicyListReq struct {
	g.Meta `path:"/admin/notifications/event-policies" method:"get" tags:"IDC Notification Event Policies" summary:"List notification event policies" dc:"Returns the bounded notify-owned event, recipient-kind, and channel policy matrix." permission:"idc:admin:notification:channel:manage"`
	Limit  int `json:"limit" in:"query" d:"100" v:"min:1|max:100" dc:"Maximum event policies to return" eg:"100"`
}

type EventPolicyItem struct {
	EventCode     string `json:"eventCode" dc:"Business notification event code" eg:"support.ticket_created"`
	Topic         string `json:"topic" dc:"Notification preference topic" eg:"support"`
	RecipientKind string `json:"recipientKind" dc:"Authoritative recipient kind" eg:"customer_contact"`
	ChannelCode   string `json:"channelCode" dc:"Notify-owned channel code" eg:"email"`
	Enabled       bool   `json:"enabled" dc:"Whether notify should materialize this event and channel" eg:"true"`
	Version       int64  `json:"version" dc:"Optimistic policy version" eg:"1"`
	UpdatedAt     int64  `json:"updatedAt" dc:"Policy update time as a Unix timestamp in milliseconds" eg:"1786752000000"`
}

type EventPolicyListRes struct {
	Items []*EventPolicyItem `json:"items" dc:"Bounded event policy matrix" eg:"[]"`
}
