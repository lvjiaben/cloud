// This file defines complete enterprise verification submission.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationSubmitReq submits legal values into one encrypted snapshot.
type EnterpriseVerificationSubmitReq struct {
	g.Meta                            `path:"/account/compliance/enterprise-verifications/{caseNo}/submission" method:"post" tags:"IDC Compliance Account" summary:"Submit enterprise verification for review" dc:"Validate one complete enterprise package, encrypt the legal snapshot, verify selected private materials, and atomically create the pending manual-review projection. Legal plaintext is cleared after the request."`
	CaseNo                            string   `json:"caseNo" v:"required|max-length:64" dc:"Owned enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	LegalName                         string   `json:"legalName" v:"required|max-length:128" dc:"Enterprise legal name consumed only for the encrypted snapshot" eg:"Example Technology Co., Ltd."`
	RegistrationCountryCode           string   `json:"registrationCountryCode" v:"required|length:2" dc:"ISO 3166-1 alpha-2 registration country; first release accepts CN enterprise identifiers" eg:"CN"`
	RegistrationIdentifier            string   `json:"registrationIdentifier" v:"required|length:18" dc:"Unified social credit code consumed only for encrypted snapshot and keyed duplicate protection" eg:"91350211M000100Y46"`
	LegalRepresentativeName           string   `json:"legalRepresentativeName" v:"required|max-length:128" dc:"Legal representative name consumed only for the encrypted snapshot" eg:"Test Representative"`
	LegalRepresentativeIdentityNumber string   `json:"legalRepresentativeIdentityNumber" v:"required|max-length:128" dc:"Legal representative identity number consumed only for the encrypted snapshot" eg:"110101199001010015"`
	AgentName                         string   `json:"agentName" v:"max-length:128" dc:"Authorized agent name; required only when applicantRoleCode is authorized_agent" eg:"Test Agent"`
	AgentIdentityNumber               string   `json:"agentIdentityNumber" v:"max-length:128" dc:"Authorized agent identity number; required only when applicantRoleCode is authorized_agent" eg:"110101199202020027"`
	IndustryCode                      string   `json:"industryCode" v:"max-length:64" dc:"Optional stable merchant-defined industry code" eg:"IDC"`
	ApplicantRoleCode                 string   `json:"applicantRoleCode" v:"required|in:legal_representative,authorized_agent" dc:"Applicant role controlling required evidence: legal_representative or authorized_agent" eg:"legal_representative"`
	MaterialNos                       []string `json:"materialNos" v:"required" dc:"One to eight selected owned material numbers; storage paths are never accepted and collection bounds are enforced by the verification service" eg:"[\"mat_0123456789abcdef0123456789abcdef\"]"`
	IdempotencyKey                    string   `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random submission retry key; never persisted verbatim" eg:"enterprise-submit-01J5M6S2R9Z8X7W6"`
}

// EnterpriseVerificationSubmitRes returns the pending manual-review state.
type EnterpriseVerificationSubmitRes struct {
	Item *EnterpriseVerificationSubmissionItem `json:"item" dc:"Pending enterprise manual-review projection" eg:"{}"`
}
