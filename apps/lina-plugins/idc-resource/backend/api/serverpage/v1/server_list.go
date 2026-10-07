// This file declares the isolated customer-page list entry. It deliberately
// keeps the existing account route separate from the required page ticket.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
)

// ServerListReq accepts only bounded paging, never a browser-selected owner.
type ServerListReq struct {
	g.Meta `path:"/account/plugin-servers" method:"get" tags:"IDC Account Servers" summary:"List my servers" dc:"Return one bounded page of servers owned by the current authenticated customer" pageBridgeCapability:"customer-servers.read" pageBridgeHost:"customer"`
	Cursor string `json:"cursor" in:"query" v:"max-length:64" dc:"Opaque cursor returned by the previous page" eg:"v1.1700000000000.42"`
	Limit  int    `json:"limit" in:"query" v:"between:1,50" d:"20" dc:"Number of servers to return, from one to fifty" eg:"20"`
}

// ServerListRes reuses the existing customer-safe account response contract.
type ServerListRes struct {
	Items      []*accountv1.ServerItem `json:"items" dc:"Customer-owned server items"`
	NextCursor string                  `json:"nextCursor" dc:"Opaque cursor for the next page, or empty when complete"`
}
