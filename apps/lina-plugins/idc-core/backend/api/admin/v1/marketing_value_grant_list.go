package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingValueGrantListReq struct {
	Page       int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta     `path:"/admin/marketing/grants" method:"get" tags:"IDC Marketing Management" summary:"List granted value" dc:"Return one bounded keyset page of recharge bonus and cashback grants." permission:"idc:admin:marketing:grant:list"`
	Type       string `json:"type" dc:"Optional grant type: recharge_bonus or cashback" eg:"recharge_bonus"`
	Status     string `json:"status" dc:"Optional grant lifecycle status" eg:"available"`
	Currency   string `json:"currency" dc:"Optional uppercase three-letter currency" eg:"CNY"`
	CustomerNo string `json:"customerNo" dc:"Optional exact customer business number" eg:"customer-001"`
	CampaignNo string `json:"campaignNo" dc:"Optional exact campaign business number" eg:"campaign-recharge-2026"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Opaque cursor from the previous response" eg:""`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum grants returned in this page" eg:"25"`
}

type MarketingValueGrantListRes struct {
	Total      int                        `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Grants     []*MarketingValueGrantItem `json:"grants" dc:"Bounded recharge bonus and cashback grant summaries" eg:"[]"`
	NextCursor string                     `json:"nextCursor" dc:"Opaque next-page cursor, or empty" eg:""`
}
