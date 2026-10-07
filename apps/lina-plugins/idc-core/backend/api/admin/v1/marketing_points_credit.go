package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingPointsCreditReq struct {
	g.Meta     `path:"/admin/marketing/points/accounts/{customerNo}/credits" method:"post" tags:"IDC Marketing Management" summary:"Credit customer points" dc:"Append one idempotent manual non-monetary points lot for an active customer." permission:"idc:admin:marketing:points:credit"`
	CustomerNo string `json:"customerNo" v:"required|max-length:128" dc:"Active customer business number" eg:"customer-001"`
	RequestNo  string `json:"requestNo" v:"required|max-length:128" dc:"Client-stable idempotency request number and manual source reference" eg:"points-credit-001"`
	Points     int64  `json:"points" v:"required|min:1|max:1000000000000" dc:"Positive non-monetary points quantity" eg:"1000"`
	ExpiresAt  *int64 `json:"expiresAt" dc:"Optional lot expiry in Unix milliseconds" eg:"1798761600000"`
	ReasonCode string `json:"reasonCode" v:"required|max-length:64" dc:"Stable operational reason code" eg:"manual_correction"`
}

type MarketingPointsCreditRes struct {
	Mutation *MarketingManagedPointsMutation `json:"mutation" dc:"Audited non-monetary points credit result" eg:"{}"`
}
