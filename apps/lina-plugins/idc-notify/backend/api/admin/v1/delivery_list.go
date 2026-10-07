package v1

import "github.com/gogf/gf/v2/frame/g"

type DeliveryListReq struct {
	g.Meta      `path:"/admin/notifications/deliveries" method:"get" tags:"IDC Notification Deliveries" summary:"List notification deliveries" dc:"Returns a bounded cursor page of authorized notification delivery projections for operational investigation." permission:"idc:admin:notification:channel:manage"`
	ChannelCode string `json:"channelCode" in:"query" v:"max-length:32" dc:"Optional exact channel filter; omit to include every channel" eg:"in_app"`
	Status      string `json:"status" in:"query" v:"max-length:40" dc:"Optional exact delivery-status filter; omit to include every status" eg:"failed"`
	Cursor      string `json:"cursor" in:"query" v:"max-length:80" dc:"Opaque cursor from the previous page; omit to read the first page" eg:"ntask_01jnotify000000000000000001"`
	Limit       int    `json:"limit" in:"query" v:"between:1,100" d:"20" dc:"Maximum records to return, from 1 to 100; defaults to 20" eg:"20"`
}
type DeliveryListRes struct {
	Items      []*DeliveryItem `json:"items" dc:"Authorized delivery projections" eg:"[]"`
	NextCursor string          `json:"nextCursor" dc:"Opaque cursor for the next page; empty when there is no next page" eg:"ntask_01jnotify000000000000000002"`
}
