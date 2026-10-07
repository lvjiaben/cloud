package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerListReq struct {
	g.Meta `path:"/account/servers" method:"get" tags:"IDC Account Servers" summary:"List my servers" dc:"Return one bounded page of servers owned by the current authenticated customer. Internal entitlement, mapping, task and supplier references are never returned."`
	Cursor string `json:"cursor" in:"query" v:"max-length:64" dc:"Opaque cursor returned by the previous page" eg:"v1.U1ZSLTAxMjM0NTY3ODlBQkNERUY"`
	Limit  int    `json:"limit" in:"query" v:"between:1,50" d:"20" dc:"Number of servers to return, from one to fifty" eg:"20"`
}

type ServerDetailReq struct {
	g.Meta   `path:"/account/servers/{serverNo}" method:"get" tags:"IDC Account Servers" summary:"Get my server" dc:"Return one current-customer-owned server by its public business number. A missing server and another customer's server have the same result."`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server business number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

type ServerItem struct {
	ServerNo         string   `json:"serverNo" dc:"Public server business number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ProductNameZhCN  string   `json:"productNameZhCn" dc:"Frozen Simplified Chinese product name" eg:"Standard compute"`
	ProductNameEnUS  string   `json:"productNameEnUs" dc:"Frozen English product name" eg:"Standard compute"`
	ChannelName      string   `json:"channelName" dc:"Customer-safe resource channel display name" eg:"Primary resources"`
	PoolName         string   `json:"poolName" dc:"Customer-safe resource pool display name" eg:"East region pool"`
	Kind             string   `json:"kind" dc:"Stable server kind code localized by the customer application" eg:"cloud_server"`
	State            string   `json:"state" dc:"Stable resource state code localized by the customer application" eg:"ready"`
	LatestOperation  string   `json:"latestOperation" dc:"Stable latest-operation state localized by the customer application" eg:"idle"`
	AvailableActions []string `json:"availableActions" dc:"Allowed action codes for the current projected capability snapshot" eg:"[\"start\",\"reboot\"]"`
	BoundAt          int64    `json:"boundAt" dc:"Unix timestamp in milliseconds when the entitlement was bound" eg:"1786464000000"`
	UpdatedAt        int64    `json:"updatedAt" dc:"Unix timestamp in milliseconds of the latest local projection" eg:"1786464300000"`
}

type ServerListRes struct {
	Items      []*ServerItem `json:"items" dc:"At most fifty customer-owned server projections" eg:"[]"`
	NextCursor string        `json:"nextCursor" dc:"Opaque next-page cursor, empty on the last page" eg:"v1.U1ZSLTAxMjM0NTY3ODlBQkNERUY"`
}

type ServerDetailRes struct {
	Item *ServerItem `json:"item" dc:"One current-customer-owned server projection" eg:"{}"`
}
