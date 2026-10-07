package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingPointsReverseReq struct {
	g.Meta               `path:"/admin/marketing/points/accounts/{customerNo}/reversals" method:"post" tags:"IDC Marketing Management" summary:"Reverse awarded points" dc:"Reverse a positive quantity from one exact original award while preserving lot and entry lineage." permission:"idc:admin:marketing:points:reverse"`
	CustomerNo           string `json:"customerNo" v:"required|max-length:128" dc:"Customer business number that owns the original award" eg:"customer-001"`
	RequestNo            string `json:"requestNo" v:"required|max-length:128" dc:"Client-stable idempotency request number and reversal source reference" eg:"points-reverse-001"`
	AwardSourceType      string `json:"awardSourceType" v:"required|max-length:24" dc:"Exact original award source type" eg:"order"`
	AwardSourceReference string `json:"awardSourceReference" v:"required|max-length:128" dc:"Exact original award source business reference" eg:"order-001"`
	Points               int64  `json:"points" v:"required|min:1|max:1000000000000" dc:"Positive non-monetary points quantity to reverse" eg:"500"`
	ReasonCode           string `json:"reasonCode" v:"required|max-length:64" dc:"Stable operational reason code" eg:"refund_correction"`
}

type MarketingPointsReverseRes struct {
	Mutation *MarketingManagedPointsMutation `json:"mutation" dc:"Audited points award reversal result" eg:"{}"`
}
