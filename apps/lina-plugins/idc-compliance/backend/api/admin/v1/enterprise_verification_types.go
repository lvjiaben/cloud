// This file defines secret-safe enterprise review API projections.

package v1

// EnterpriseVerificationCaseItem is one bounded queue row.
type EnterpriseVerificationCaseItem struct {
	CaseNo               string `json:"caseNo" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	CustomerNo           string `json:"customerNo" dc:"Opaque customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	StatusCode           string `json:"statusCode" dc:"Enterprise verification status code: pending, verifying, review_required, verified, rejected, expired, or canceled" eg:"verifying"`
	MaskedLegalName      string `json:"maskedLegalName" dc:"Masked enterprise legal name" eg:"E******"`
	MaskedRegistrationID string `json:"maskedRegistrationId" dc:"Masked enterprise registration identifier" eg:"91************1234"`
	ApplicantRoleCode    string `json:"applicantRoleCode" dc:"Applicant role code: legal_representative or authorized_agent" eg:"legal_representative"`
	Reverification       bool   `json:"reverification" dc:"Whether this case reviews an existing verified legal profile change" eg:"false"`
	DecisionReasonCode   string `json:"decisionReasonCode" dc:"Stable decision reason code from the documented approval, duplicate-risk, or rejection catalog; empty before a decision" eg:"materials_incomplete"`
	SubmittedAt          int64  `json:"submittedAt" dc:"Submission Unix timestamp in milliseconds" eg:"1786464000000"`
	UpdatedAt            int64  `json:"updatedAt" dc:"Latest case update Unix timestamp in milliseconds" eg:"1786464060000"`
	Version              int64  `json:"version" dc:"Optimistic concurrency version" eg:"3"`
}

// EnterpriseVerificationMaskedLegalProfile contains no raw registration or identity number.
type EnterpriseVerificationMaskedLegalProfile struct {
	LegalName               string `json:"legalName" dc:"Masked enterprise legal name" eg:"E******"`
	RegistrationCountryCode string `json:"registrationCountryCode" dc:"Registration country code" eg:"CN"`
	RegistrationIdentifier  string `json:"registrationIdentifier" dc:"Masked registration identifier" eg:"91************1234"`
	LegalRepresentativeName string `json:"legalRepresentativeName" dc:"Masked legal representative name" eg:"R*****"`
	LegalRepresentativeID   string `json:"legalRepresentativeId" dc:"Masked legal representative identity number" eg:"**************1234"`
	AgentPresent            bool   `json:"agentPresent" dc:"Whether an authorized agent is present" eg:"false"`
	AgentName               string `json:"agentName" dc:"Masked authorized-agent name; empty when absent" eg:"A****"`
	AgentIdentityNumber     string `json:"agentIdentityNumber" dc:"Masked authorized-agent identity number; empty when absent" eg:"**************5678"`
	IndustryCode            string `json:"industryCode" dc:"Optional industry code" eg:"IDC"`
	ApplicantRoleCode       string `json:"applicantRoleCode" dc:"Applicant role code: legal_representative or authorized_agent" eg:"legal_representative"`
}

// EnterpriseVerificationMaterialItem excludes paths, digests, keys, and access URLs.
type EnterpriseVerificationMaterialItem struct {
	MaterialNo     string `json:"materialNo" dc:"Opaque material number" eg:"mat_0123456789abcdef0123456789abcdef"`
	MaterialType   string `json:"materialTypeCode" dc:"Enterprise material type code: business_license, legal_representative_id, agent_id, authorization_letter, organization_certificate, or other" eg:"business_license"`
	SequenceNo     int    `json:"sequenceNo" dc:"Material sequence number" eg:"1"`
	MediaType      string `json:"mediaType" dc:"Verified original media type" eg:"application/pdf"`
	ByteSize       int64  `json:"byteSize" dc:"Original byte size" eg:"204800"`
	StatusCode     string `json:"statusCode" dc:"Material retention status code: available or legal_hold" eg:"available"`
	RetentionUntil *int64 `json:"retentionUntil" dc:"Retention deadline Unix timestamp in milliseconds; null when not scheduled" eg:"1818000000000"`
	LegalHold      bool   `json:"legalHold" dc:"Whether legal hold prevents ordinary disposal" eg:"false"`
}

// EnterpriseVerificationReviewResult is returned by every review transition.
type EnterpriseVerificationReviewResult struct {
	CaseNo             string `json:"caseNo" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	StatusCode         string `json:"statusCode" dc:"Resulting case status: verifying, verified, rejected, or review_required" eg:"verified"`
	DecisionReasonCode string `json:"decisionReasonCode" dc:"Stable decision reason code; empty while verifying" eg:"manual_review_passed"`
	Version            int64  `json:"version" dc:"Resulting optimistic concurrency version" eg:"4"`
	Reused             bool   `json:"reused" dc:"Whether the same idempotent result was replayed" eg:"false"`
}
