// Privacy assessment API types define safe governance projections and complete draft input. These contracts exclude personal data, secrets, and private object locations.

package v1

// PrivacyAssessmentItem is one safe governance-text projection. It never
// contains personal data, private object locations, access tokens, or secrets.
type PrivacyAssessmentItem struct {
	AssessmentNo             string   `json:"assessmentNo" dc:"Stable assessment number" eg:"PIA_0123456789abcdef0123456789abcdef"`
	AssessmentTypeCode       string   `json:"assessmentTypeCode" dc:"Assessment type" eg:"personal_verification"`
	Version                  int64    `json:"version" dc:"Published content version" eg:"1"`
	StatusCode               string   `json:"statusCode" dc:"Draft, published, or superseded" eg:"published"`
	Purpose                  string   `json:"purpose" dc:"Processing purpose" eg:"Verify the account holder before first payment"`
	Necessity                string   `json:"necessity" dc:"Why this processing is necessary" eg:"Required to meet local identity obligations"`
	Impact                   string   `json:"impact" dc:"Expected impact on the customer" eg:"Identity materials are checked by the configured local supplier"`
	Recipients               string   `json:"recipients" dc:"Recipient description" eg:"The current IDC merchant and its configured verification supplier"`
	StorageLocation          string   `json:"storageLocation" dc:"Storage location description" eg:"This installation and its configured local supplier"`
	RetentionDescription     string   `json:"retentionDescription" dc:"Retention explanation" eg:"Uses the active personal verification material policy"`
	RightsEntry              string   `json:"rightsEntry" dc:"Customer rights entry" eg:"Privacy and authorization page in the customer center"`
	MerchantRole             string   `json:"merchantRole" dc:"Merchant processing role" eg:"Personal information processor"`
	ThirdPartyRole           string   `json:"thirdPartyRole" dc:"Third-party processing role" eg:"Entrusted verification processor"`
	ProcessingInstruction    string   `json:"processingInstruction" dc:"Bounded processing instruction" eg:"Process only the submitted verification request"`
	SubcontractingBoundary   string   `json:"subcontractingBoundary" dc:"Subcontracting boundary" eg:"No subcontracting beyond the configured supplier contract"`
	AuditEvidenceDescription string   `json:"auditEvidenceDescription" dc:"Audit evidence description" eg:"Consent, request, result, access, and deletion audit records"`
	AIAllowedFields          []string `json:"aiAllowedFields" dc:"Explicit safe AI projection field codes" eg:"[]"`
	CrossBorderTransfer      bool     `json:"crossBorderTransfer" dc:"Always false in the first release" eg:"false"`
	ConfirmedByUserID        int      `json:"confirmedByUserId" dc:"Publishing administrator identifier; zero for drafts" eg:"1"`
	ConfirmedAt              int64    `json:"confirmedAt" dc:"Publish time in Unix milliseconds; zero for drafts" eg:"1787040000000"`
	CreatedAt                int64    `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1787040000000"`
	UpdatedAt                int64    `json:"updatedAt" dc:"Last update time in Unix milliseconds" eg:"1787040000000"`
	RowVersion               int64    `json:"rowVersion" dc:"Optimistic concurrency version" eg:"1"`
}

// PrivacyAssessmentDraftPayload is the complete safe governance text saved as one draft.
type PrivacyAssessmentDraftPayload struct {
	AssessmentTypeCode       string   `json:"assessmentTypeCode" v:"required|max-length:40" dc:"One of the six assessment types" eg:"personal_verification"`
	Purpose                  string   `json:"purpose" v:"required|max-length:2000" dc:"Processing purpose" eg:"Verify the account holder before first payment"`
	Necessity                string   `json:"necessity" v:"required|max-length:2000" dc:"Why processing is necessary" eg:"Required to meet local identity obligations"`
	Impact                   string   `json:"impact" v:"required|max-length:2000" dc:"Expected customer impact" eg:"Verification may be unavailable after withdrawal"`
	Recipients               string   `json:"recipients" v:"required|max-length:2000" dc:"Data recipient description" eg:"The configured local verification supplier"`
	StorageLocation          string   `json:"storageLocation" v:"required|max-length:1000" dc:"Storage location description" eg:"This installation and its configured local supplier"`
	RetentionDescription     string   `json:"retentionDescription" v:"required|max-length:1000" dc:"Retention policy description" eg:"Uses the active personal verification material policy"`
	RightsEntry              string   `json:"rightsEntry" v:"required|max-length:1000" dc:"Customer rights entry" eg:"Privacy and authorization page in the customer center"`
	MerchantRole             string   `json:"merchantRole" v:"required|max-length:1000" dc:"Merchant processing role" eg:"Personal information processor"`
	ThirdPartyRole           string   `json:"thirdPartyRole" v:"required|max-length:1000" dc:"Third-party processing role" eg:"Entrusted verification processor"`
	ProcessingInstruction    string   `json:"processingInstruction" v:"required|max-length:2000" dc:"Bounded processing instruction" eg:"Process only the submitted verification request"`
	SubcontractingBoundary   string   `json:"subcontractingBoundary" v:"required|max-length:2000" dc:"Subcontracting boundary" eg:"No subcontracting beyond the configured supplier contract"`
	AuditEvidenceDescription string   `json:"auditEvidenceDescription" v:"required|max-length:2000" dc:"Audit evidence description" eg:"Consent, request, result, access, and deletion audit records"`
	AIAllowedFields          []string `json:"aiAllowedFields" dc:"At most seven safe AI field codes; collection bounds are enforced by the privacy assessment service" eg:"[]"`
	ExpectedRowVersion       int64    `json:"expectedRowVersion" v:"min:0" dc:"Zero to create a draft; current row version to update" eg:"0"`
}
