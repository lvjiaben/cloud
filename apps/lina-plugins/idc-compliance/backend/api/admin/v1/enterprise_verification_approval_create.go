// This file defines the server-authoritative enterprise-verification approval command.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationApprovalCreateReq approves server-authoritative legal values.
type EnterpriseVerificationApprovalCreateReq struct {
	g.Meta          `path:"/admin/compliance/enterprise-verifications/{caseNo}/approvals" method:"post" tags:"IDC Compliance Enterprise Review" summary:"Approve an enterprise verification case" dc:"Approve only the authenticated encrypted legal snapshot selected by the server. The request cannot supply or overwrite legal values; duplicate registration identifiers fail into review-required." permission:"idc:admin:compliance:enterprise-verification:approve"`
	CaseNo          string `json:"caseNo" v:"required|max-length:64" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed case version" eg:"3"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated retry key carried only by this JSON command" eg:"approval-20260813-0001"`
}

// EnterpriseVerificationApprovalCreateRes wraps the approval or duplicate-risk result.
type EnterpriseVerificationApprovalCreateRes struct {
	Item *EnterpriseVerificationReviewResult `json:"item" dc:"Verified or review-required result" eg:"{}"`
}
