package v1

import "github.com/gogf/gf/v2/frame/g"

type DeliveryGetReq struct {
	g.Meta `path:"/admin/notifications/deliveries/{taskNo}" method:"get" tags:"IDC Notification Deliveries" summary:"Get notification delivery" dc:"Returns one authorized delivery projection without exposing raw recipient or provider-secret data." permission:"idc:admin:notification:channel:manage"`
	TaskNo string `json:"taskNo" in:"path" v:"required|max-length:80" dc:"Opaque notification delivery task number" eg:"ntask_01jnotify000000000000000001"`
}
type DeliveryGetRes struct {
	Item *DeliveryItem `json:"item" dc:"Authorized notification delivery projection" eg:"{}"`
}
