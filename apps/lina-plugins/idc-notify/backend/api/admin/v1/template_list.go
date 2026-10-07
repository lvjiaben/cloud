package v1

import "github.com/gogf/gf/v2/frame/g"

type TemplateListReq struct {
	g.Meta      `path:"/admin/notifications/templates" method:"get" tags:"IDC Notification Templates" summary:"List notification template versions" dc:"Returns a bounded newest-first list of notification-template versions, optionally filtered by event and channel." permission:"idc:admin:notification:channel:manage"`
	EventCode   string `json:"eventCode" in:"query" v:"max-length:96" dc:"Optional exact event-code filter; omit to include every event" eg:"support.ticket_updated"`
	ChannelCode string `json:"channelCode" in:"query" v:"max-length:32" dc:"Optional exact channel filter; omit to include every channel" eg:"in_app"`
	Limit       int    `json:"limit" in:"query" v:"between:1,100" d:"20" dc:"Maximum template versions to return, from 1 to 100; defaults to 20" eg:"20"`
}
type TemplateListRes struct {
	Items []*TemplateItem `json:"items" dc:"Notification template versions matching the filters" eg:"[]"`
}
