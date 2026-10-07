package v1

import "github.com/gogf/gf/v2/frame/g"

type DeliveryListReq struct {
	g.Meta `path:"/account/notifications/deliveries" method:"get" tags:"IDC Account Notifications" summary:"List current customer's notification deliveries" dc:"Returns a bounded cursor page of delivery records visible to the currently authenticated customer."`
	Cursor string `json:"cursor" in:"query" v:"max-length:80" dc:"Opaque cursor from the previous page; omit to read the first page" eg:"ntask_01jnotify000000000000000001"`
	Limit  int    `json:"limit" in:"query" v:"between:1,100" d:"20" dc:"Maximum records to return, from 1 to 100; defaults to 20" eg:"20"`
}
type DeliveryListRes struct {
	Items      []*DeliveryItem `json:"items" dc:"Delivery records visible to the current customer" eg:"[]"`
	NextCursor string          `json:"nextCursor" dc:"Opaque cursor for the next page; empty when there is no next page" eg:"ntask_01jnotify000000000000000002"`
}
