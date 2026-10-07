package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingPointsAccountListReq struct {
	Page       int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta     `path:"/admin/marketing/points/accounts" method:"get" tags:"IDC Marketing Management" summary:"List points accounts" dc:"Return one bounded keyset page of non-monetary points accounts." permission:"idc:admin:marketing:points:list"`
	CustomerNo string `json:"customerNo" dc:"Optional exact customer business number" eg:"customer-001"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Opaque cursor from the previous response" eg:""`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum accounts returned in this page" eg:"25"`
}

type MarketingPointsAccountListRes struct {
	Total      int                           `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Accounts   []*MarketingPointsAccountItem `json:"accounts" dc:"Bounded non-monetary points account summaries" eg:"[]"`
	NextCursor string                        `json:"nextCursor" dc:"Opaque next-page cursor, or empty" eg:""`
}
