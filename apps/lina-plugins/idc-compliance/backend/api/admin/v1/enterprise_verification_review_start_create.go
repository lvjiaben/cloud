// This file defines the idempotent enterprise-verification review-start command.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationReviewStartCreateReq starts or restarts manual review.
type EnterpriseVerificationReviewStartCreateReq struct {
	g.Meta          `path:"/admin/compliance/enterprise-verifications/{caseNo}/review-starts" method:"post" tags:"IDC Compliance Enterprise Review" summary:"Start enterprise verification review" dc:"Move one pending or review-required enterprise case into verifying with exact permission, optimistic concurrency, idempotency, and immutable audit." permission:"idc:admin:compliance:enterprise-verification:review"`
	CaseNo          string `json:"caseNo" v:"required|max-length:64" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed case version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated retry key carried only by this JSON command" eg:"review-start-20260813-0001"`
}

// EnterpriseVerificationReviewStartCreateRes wraps the transition result.
type EnterpriseVerificationReviewStartCreateRes struct {
	Item *EnterpriseVerificationReviewResult `json:"item" dc:"Resulting review state" eg:"{}"`
}
