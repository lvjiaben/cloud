// This file defines the authenticated customer's bounded contract list API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerContractListReq lists only the authenticated customer's contracts.
type CustomerContractListReq struct {
	g.Meta `path:"/account/compliance/contracts" method:"get" tags:"IDC Compliance Account" summary:"List my contracts" dc:"Return one bounded stable-cursor page owned by the authenticated customer. The request never accepts a customer or tenant identifier and never performs supplier I/O."`
	Cursor string `json:"cursor" v:"max-length:256" dc:"Opaque cursor returned by the previous page" eg:"eyJjcmVhdGVkQXQiOiIyMDI2LTA4LTE3VDEwOjAwOjAwWiIsImlkIjoxfQ"`
	Limit  int    `json:"limit" v:"min:1|max:50" d:"20" dc:"Bounded page size from 1 to 50" eg:"20"`
}

// CustomerContractListRes returns one bounded page without an expensive exact total.
type CustomerContractListRes struct {
	Items      []CustomerContractItem `json:"items" dc:"Bounded customer-owned contract page" eg:"[]"`
	NextCursor string                 `json:"nextCursor" dc:"Opaque next-page cursor; empty when no next page exists" eg:"eyJjcmVhdGVkQXQiOiIyMDI2LTA4LTE3VDEwOjAwOjAwWiIsImlkIjoxfQ"`
}
