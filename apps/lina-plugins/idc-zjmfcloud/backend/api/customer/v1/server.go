// This file defines the governed customer server list and detail HTTP contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ServerListReq selects one bounded page without accepting a customer identity.
type ServerListReq struct {
	g.Meta `path:"/customer-servers" method:"get" tags:"ZJMFCloud Customer Servers" summary:"List my ZJMFCloud servers" dc:"Return one bounded page of customer-safe servers for the current authenticated customer through the resource owner." access:"login" pageBridgeCapability:"customer-servers.read"`
	Cursor string `json:"cursor" in:"query" v:"max-length:128" dc:"Opaque cursor returned by the previous page" eg:"eyJvZmZzZXQiOjIwfQ"`
	Limit  int    `json:"limit" in:"query" v:"between:1,50" d:"20" dc:"Number of customer servers to return, from one to fifty" eg:"20"`
}

// ServerDetailReq selects one server by its opaque public routing key.
type ServerDetailReq struct {
	g.Meta   `path:"/customer-server" method:"get" tags:"ZJMFCloud Customer Servers" summary:"Get my ZJMFCloud server" dc:"Return one customer-safe server projection after the resource owner verifies current-customer ownership." access:"login" pageBridgeCapability:"customer-servers.read"`
	ServerNo string `json:"serverNo" in:"query" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server number returned by the customer server list" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

// ServerItem is the customer-safe resource projection returned to the isolated page.
type ServerItem struct {
	ServerNo         string   `json:"serverNo" dc:"Opaque server routing key; customer pages must not render it as a business identifier" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ProductNameZhCN  string   `json:"productNameZhCn" dc:"Frozen Simplified Chinese product name" eg:"Business Cloud 4C8G"`
	ProductNameEnUS  string   `json:"productNameEnUs" dc:"Frozen English product name" eg:"Business Cloud 4C8G"`
	ChannelName      string   `json:"channelName" dc:"Customer-safe resource channel display name" eg:"East China channel"`
	PoolName         string   `json:"poolName" dc:"Customer-safe resource pool display name" eg:"Shanghai pool"`
	Kind             string   `json:"kind" dc:"Stable server kind code localized by this plugin page" eg:"cloud_server"`
	State            string   `json:"state" dc:"Stable server state code localized by this plugin page" eg:"ready"`
	LatestOperation  string   `json:"latestOperation" dc:"Stable latest-operation state localized by this plugin page" eg:"idle"`
	AvailableActions []string `json:"availableActions" dc:"Customer-safe action codes for future governed actions" eg:"reboot"`
	BoundAt          int64    `json:"boundAt" dc:"Unix timestamp in milliseconds when the server was bound" eg:"1777000000000"`
	UpdatedAt        int64    `json:"updatedAt" dc:"Unix timestamp in milliseconds of the latest local projection" eg:"1777000100000"`
}

// ServerListRes contains one page and an opaque continuation cursor.
type ServerListRes struct {
	Items      []*ServerItem `json:"items" dc:"At most fifty customer-owned server projections" eg:"[]"`
	NextCursor string        `json:"nextCursor" dc:"Opaque next-page cursor, empty on the last page" eg:"eyJvZmZzZXQiOjIwfQ"`
}

// ServerDetailRes contains one ownership-checked server projection.
type ServerDetailRes struct {
	Item *ServerItem `json:"item" dc:"One customer-owned server projection" eg:"{}"`
}
