// This file defines the bounded marketing campaign management queue.

package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingCampaignListReq struct {
	Page            int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta          `path:"/admin/marketing/campaigns" method:"get" tags:"IDC Marketing Management" summary:"List marketing campaigns" dc:"Return one bounded keyset page of campaign summaries, quota counters, and rule counts." permission:"idc:admin:marketing:campaign:list"`
	NamePrefix      string `json:"namePrefix" dc:"Optional literal campaign code, Simplified Chinese name, or English name prefix; percent and underscore are not wildcards" eg:"summer"`
	Type            string `json:"type" dc:"Optional campaign type: discount, coupon, voucher, points, recharge_bonus, cashback, or customer_price" eg:"cashback"`
	Status          string `json:"status" dc:"Optional campaign status: draft, scheduled, active, paused, ended, or archived" eg:"active"`
	TargetSegmentNo string `json:"targetSegmentNo" dc:"Optional exact target segment business number" eg:"segment-loyal"`
	Cursor          string `json:"cursor" v:"max-length:128" dc:"Opaque cursor from the previous response; empty requests the first page" eg:""`
	Limit           int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum campaigns returned in this page" eg:"25"`
}

type MarketingCampaignListRes struct {
	Total      int                                `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Campaigns  []*MarketingCampaignManagementItem `json:"campaigns" dc:"Bounded campaign workbench summaries" eg:"[]"`
	NextCursor string                             `json:"nextCursor" dc:"Opaque cursor for the next page, or empty when complete" eg:""`
}
