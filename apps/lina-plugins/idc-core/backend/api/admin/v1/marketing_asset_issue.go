package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingAssetIssueReq struct {
	g.Meta     `path:"/admin/marketing/assets/issue" method:"post" tags:"IDC Marketing Management" summary:"Issue promotion asset" dc:"Issue one single-use coupon or voucher from an active campaign rule to an active eligible customer." permission:"idc:admin:marketing:asset:issue"`
	RequestNo  string `json:"requestNo" v:"required|max-length:128" dc:"Client-stable idempotency request number" eg:"marketing-issue-001"`
	CampaignNo string `json:"campaignNo" v:"required|max-length:128" dc:"Active coupon or voucher campaign business number" eg:"campaign-summer-2026"`
	BenefitNo  string `json:"benefitNo" v:"required|max-length:128" dc:"Active monetary benefit rule business number" eg:"benefit-summer-voucher"`
	CustomerNo string `json:"customerNo" v:"required|max-length:128" dc:"Active eligible customer business number" eg:"customer-001"`
	ReasonCode string `json:"reasonCode" v:"required|max-length:64" dc:"Stable operational reason code" eg:"customer_service"`
}

type MarketingAssetIssueRes struct {
	Mutation *MarketingManagedAssetMutation `json:"mutation" dc:"Audited promotion asset issuance result" eg:"{}"`
}
