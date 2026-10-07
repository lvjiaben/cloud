package v1

import "github.com/gogf/gf/v2/frame/g"

type EventListReq struct {
	g.Meta `path:"/admin/notifications/events" method:"get" tags:"IDC Notification Templates" summary:"List notification events" dc:"Returns the bounded registered event catalog used when editing and previewing notification templates." permission:"idc:admin:notification:channel:manage"`
}
type EventListRes struct {
	Items []*EventItem `json:"items" dc:"Registered notification event definitions" eg:"[]"`
}
