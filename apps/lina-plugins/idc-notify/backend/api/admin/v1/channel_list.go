package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelListReq struct {
	g.Meta `path:"/admin/notifications/channels" method:"get" tags:"IDC Notification Channels" summary:"List notification channels" dc:"Returns the bounded installed-channel registry with configuration state, masked credential evidence, and pending-query counts." permission:"idc:admin:notification:channel:manage"`
}
type ChannelListRes struct {
	Items []*ChannelItem `json:"items" dc:"Installed notification channels" eg:"[]"`
}
