// This file defines the bounded local server renewal directory API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerRenewalListReq struct {
	Page   int `json:"page" in:"query" v:"between:1,1000000" dc:"One-based page number; cannot be combined with a cursor" eg:"1"`
	g.Meta `path:"/admin/resource/server-renewals" method:"get" tags:"IDC Server Renewals" summary:"List renewable server candidates" dc:"Return one bounded page from the resource plugin's own server entitlement projection. The endpoint does not call the renewal core and never returns customer or entitlement identifiers." permission:"idc:admin:resource:server-batch:view"`
	Cursor string `json:"cursor" in:"query" v:"max-length:36" dc:"Server number continuation cursor returned by the previous page" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Limit  int    `json:"limit" in:"query" v:"between:1,100" d:"25" dc:"Maximum returned server projections; defaults to 25" eg:"25"`
}

type ServerRenewalListRes struct {
	Total      int                       `json:"total" dc:"Exact total within the current staff scope and filters" eg:"120"`
	Items      []*ServerRenewalCandidate `json:"items" dc:"At most 100 resource-owned server candidates"`
	NextCursor string                    `json:"nextCursor" dc:"Continuation server number; empty on the last page"`
}

type ServerRenewalCandidate struct {
	ServerNo        string `json:"serverNo" dc:"Stable public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ProductNameZhCN string `json:"productNameZhCN" dc:"Frozen customer-safe Simplified Chinese product name" eg:"Standard Cloud Server"`
	ProductNameEnUS string `json:"productNameEnUS" dc:"Frozen customer-safe English product name" eg:"Standard Cloud Server"`
}
