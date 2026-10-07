package v1

import "github.com/gogf/gf/v2/frame/g"

type CommerceOrderRiskReviewRejectionReq struct {
	g.Meta         `path:"/admin/commerce/order-risk-reviews/{reviewNo}/rejection" method:"put" tags:"IDC Commerce Management" summary:"Reject order risk review" dc:"Reject one pending order risk review under its exact staff permission and atomically cancel the unpaid order." permission:"idc:admin:commerce:risk-review:reject"`
	ReviewNo       string `json:"reviewNo" v:"required|max-length:40" dc:"Pending order risk-review business number" eg:"review-risk-001"`
	RequestNo      string `json:"requestNo" v:"required|min-length:8|max-length:128" dc:"Client-stable idempotency request number" eg:"risk-reject-001"`
	EvidenceDigest string `json:"evidenceDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"Lowercase SHA-256 digest of the reviewed evidence package" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

type CommerceOrderRiskReviewRejectionRes struct {
	Decision *CommerceOrderRiskReviewDecisionItem `json:"decision" dc:"Audited committed risk-review rejection and order cancellation" eg:"{}"`
}
