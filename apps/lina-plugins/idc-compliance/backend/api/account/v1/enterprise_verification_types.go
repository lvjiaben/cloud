// This file defines customer-safe enterprise-verification API projections.

package v1

// EnterpriseVerificationDraftItem is the safe draft projection.
type EnterpriseVerificationDraftItem struct {
	CaseNo               string `json:"caseNo" dc:"Opaque enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	StatusCode           string `json:"statusCode" dc:"Case status; this projection returns draft" eg:"draft"`
	LegalChangeRequestNo string `json:"legalChangeRequestNo" dc:"Opaque legal-change request number when re-verifying" eg:"LCR_0123456789abcdef0123456789abcdef"`
	Reverification       bool   `json:"reverification" dc:"Whether the draft reviews a critical legal profile change" eg:"false"`
	Reused               bool   `json:"reused" dc:"Whether the exact draft request already existed" eg:"false"`
}

// EnterpriseVerificationMaterialReservationItem contains only the opaque
// server-assigned upload number and bounded reservation state.
type EnterpriseVerificationMaterialReservationItem struct {
	MaterialNo       string `json:"materialNo" dc:"Opaque server-assigned material number" eg:"mat_0123456789abcdef0123456789abcdef"`
	MaterialTypeCode string `json:"materialTypeCode" dc:"Reserved evidence category: business_license, legal_representative_id, agent_id, authorization_letter, organization_certificate, or other" eg:"business_license"`
	StatusCode       string `json:"statusCode" dc:"Reservation status: reserved or completed" eg:"reserved"`
	Reused           bool   `json:"reused" dc:"Whether the exact reservation command already existed" eg:"false"`
}

// EnterpriseVerificationMaterialItem contains no storage routing metadata.
type EnterpriseVerificationMaterialItem struct {
	MaterialNo       string `json:"materialNo" dc:"Opaque material number" eg:"mat_0123456789abcdef0123456789abcdef"`
	MaterialTypeCode string `json:"materialTypeCode" dc:"Evidence category: business_license, legal_representative_id, agent_id, authorization_letter, organization_certificate, or other" eg:"business_license"`
	SequenceNo       int    `json:"sequenceNo" dc:"Immutable upload sequence within the case" eg:"1"`
	MediaType        string `json:"mediaType" dc:"Content-sniffed media type" eg:"application/pdf"`
	ByteSize         int64  `json:"byteSize" dc:"Original byte size without content or filename" eg:"102400"`
	StatusCode       string `json:"statusCode" dc:"Confirmed material status" eg:"available"`
	Reused           bool   `json:"reused" dc:"Whether the exact upload request was recovered or replayed" eg:"false"`
}

// EnterpriseVerificationSubmissionItem is the safe pending projection.
type EnterpriseVerificationSubmissionItem struct {
	CaseNo         string `json:"caseNo" dc:"Opaque enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	StatusCode     string `json:"statusCode" dc:"Manual review status; this projection returns pending" eg:"pending"`
	Reverification bool   `json:"reverification" dc:"Whether the current verified projection remains preserved during review" eg:"false"`
	Reused         bool   `json:"reused" dc:"Whether the exact submission was safely replayed" eg:"false"`
}

// EnterpriseVerificationAccountCaseItem is the latest customer-safe compliance case.
type EnterpriseVerificationAccountCaseItem struct {
	CaseNo               string                               `json:"caseNo" dc:"Opaque latest enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	StatusCode           string                               `json:"statusCode" dc:"Latest case status: draft, pending, verifying, review_required, verified, rejected, expired, or canceled" eg:"pending"`
	DecisionReasonCode   string                               `json:"decisionReasonCode" dc:"Stable customer-actionable decision reason, empty before a decision" eg:"materials_incomplete"`
	LegalChangeRequestNo string                               `json:"legalChangeRequestNo" dc:"Opaque legal-change request number during re-verification" eg:"LCR_0123456789abcdef0123456789abcdef"`
	Reverification       bool                                 `json:"reverification" dc:"Whether the case reviews a critical legal-profile change" eg:"false"`
	SubmittedAt          int64                                `json:"submittedAt" dc:"Submission time as Unix milliseconds, or zero before submission" eg:"1786502400000"`
	VerifiedAt           int64                                `json:"verifiedAt" dc:"Verification time as Unix milliseconds, or zero before approval" eg:"1786502400000"`
	RejectedAt           int64                                `json:"rejectedAt" dc:"Rejection time as Unix milliseconds, or zero when not rejected" eg:"0"`
	ExpiresAt            int64                                `json:"expiresAt" dc:"Expiry time as Unix milliseconds, or zero when not scheduled" eg:"0"`
	UpdatedAt            int64                                `json:"updatedAt" dc:"Last case update time as Unix milliseconds" eg:"1786502400000"`
	Version              int64                                `json:"version" dc:"Current optimistic case version" eg:"2"`
	Materials            []EnterpriseVerificationMaterialItem `json:"materials" dc:"At most eight customer-owned safe material metadata items; no filename, object reference, digest, key, or download URL" eg:"[]"`
}

// EnterpriseVerificationAccountProfileItem is the core-owned current legal projection.
type EnterpriseVerificationAccountProfileItem struct {
	VerificationCaseNo           string `json:"verificationCaseNo" dc:"Opaque case number backing the current core legal projection" eg:"ent_0123456789abcdef0123456789abcdef"`
	VerificationStatusCode       string `json:"verificationStatusCode" dc:"Current enterprise identity status: unverified, pending, verified, rejected, expired, or revoked" eg:"verified"`
	LegalName                    string `json:"legalName" dc:"Current customer-owned enterprise legal name; empty until verified" eg:"Example Technology Co., Ltd."`
	RegistrationCountryCode      string `json:"registrationCountryCode" dc:"Current enterprise registration country code" eg:"CN"`
	RegistrationIdentifierMasked string `json:"registrationIdentifierMasked" dc:"Masked current enterprise registration identifier" eg:"91************0Y46"`
	IndustryCode                 string `json:"industryCode" dc:"Optional current enterprise industry code" eg:"I6510"`
	VerifiedAt                   int64  `json:"verifiedAt" dc:"Current projection verification time as Unix milliseconds" eg:"1786502400000"`
	ExpiresAt                    int64  `json:"expiresAt" dc:"Current projection expiry time as Unix milliseconds, or zero" eg:"0"`
	ProfileVersion               int64  `json:"profileVersion" dc:"Current enterprise legal-profile version" eg:"7"`
}

// EnterpriseLegalChangeItem is a category-only core workflow projection.
type EnterpriseLegalChangeItem struct {
	RequestNo             string   `json:"requestNo" dc:"Opaque enterprise legal-change request number" eg:"LCR_0123456789abcdef0123456789abcdef"`
	StatusCode            string   `json:"statusCode" dc:"Legal-change status: submitted, under_review, approved, rejected, or cancelled" eg:"submitted"`
	ReasonCode            string   `json:"reasonCode" dc:"Closed legal-change reason code" eg:"enterprise_registration_change"`
	FieldCodes            []string `json:"fieldCodes" dc:"Changed legal categories: legal_name and/or legal_identifier; no replacement values" eg:"[\"legal_name\",\"legal_identifier\"]"`
	VerificationCaseNo    string   `json:"verificationCaseNo" dc:"Opaque currently verified case retained during review" eg:"ent_0123456789abcdef0123456789abcdef"`
	CurrentProfileVersion int64    `json:"currentProfileVersion" dc:"Verified profile version against which the change was opened" eg:"7"`
	SubmittedAt           int64    `json:"submittedAt" dc:"Legal-change submission time as Unix milliseconds" eg:"1786502400000"`
}

// EnterpriseVerificationAccountStateItem combines two owner-safe read projections.
type EnterpriseVerificationAccountStateItem struct {
	PolicyVersion     string                                    `json:"policyVersion" dc:"Current enterprise verification notice version" eg:"1"`
	Case              *EnterpriseVerificationAccountCaseItem    `json:"case" dc:"Latest compliance-owned case, null before the first draft" eg:"{}"`
	Profile           *EnterpriseVerificationAccountProfileItem `json:"profile" dc:"Current core-owned enterprise legal projection" eg:"{}"`
	ActiveLegalChange *EnterpriseLegalChangeItem                `json:"activeLegalChange" dc:"One active category-only legal-change request, null otherwise" eg:"{}"`
}
