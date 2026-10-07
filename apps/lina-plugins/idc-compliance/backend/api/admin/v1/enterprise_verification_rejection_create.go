// This file defines the actionable enterprise-verification rejection command.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationRejectionCreateReq rejects with one actionable closed reason.
type EnterpriseVerificationRejectionCreateReq struct {
	g.Meta          `path:"/admin/compliance/enterprise-verifications/{caseNo}/rejections" method:"post" tags:"IDC Compliance Enterprise Review" summary:"Reject an enterprise verification case" dc:"Reject one verifying or review-required case with a stable customer-actionable reason code, optimistic concurrency, idempotency, and immutable audit." permission:"idc:admin:compliance:enterprise-verification:reject"`
	CaseNo          string `json:"caseNo" v:"required|max-length:64" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed case version" eg:"3"`
	ReasonCode      string `json:"reasonCode" v:"required|max-length:64" dc:"Closed rejection reason: materials_incomplete, materials_unreadable, legal_information_mismatch, registration_identifier_invalid, representative_identity_mismatch, agent_authorization_invalid, registration_status_unverifiable, or business_scope_unsupported" eg:"materials_incomplete"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated retry key carried only by this JSON command" eg:"rejection-20260813-0001"`
}

// EnterpriseVerificationRejectionCreateRes wraps the rejection result.
type EnterpriseVerificationRejectionCreateRes struct {
	Item *EnterpriseVerificationReviewResult `json:"item" dc:"Rejected result" eg:"{}"`
}
