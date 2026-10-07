package v1

import "github.com/gogf/gf/v2/frame/g"

type CommerceOrderRiskReviewApprovalReq struct {
	g.Meta         `path:"/admin/commerce/order-risk-reviews/{reviewNo}/approval" method:"put" tags:"IDC Commerce Management" summary:"Approve order risk review" dc:"Approve one pending order risk review under its exact staff permission while preserving independent inventory and fulfillment gates." permission:"idc:admin:commerce:risk-review:approve"`
	ReviewNo       string `json:"reviewNo" v:"required|max-length:40" dc:"Pending order risk-review business number" eg:"review-risk-001"`
	RequestNo      string `json:"requestNo" v:"required|min-length:8|max-length:128" dc:"Client-stable idempotency request number" eg:"risk-approve-001"`
	EvidenceDigest string `json:"evidenceDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"Lowercase SHA-256 digest of the reviewed evidence package" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type CommerceOrderRiskReviewApprovalRes struct {
	Decision *CommerceOrderRiskReviewDecisionItem `json:"decision" dc:"Audited committed risk-review approval" eg:"{}"`
}
