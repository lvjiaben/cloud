// Customer privacy API types expose plain-language notices and customer-owned state. They exclude private storage references and other customers' identifiers.

package v1

// CustomerPrivacyNotice is one published plain-language privacy notice.
type CustomerPrivacyNotice struct {
	AssessmentTypeCode   string `json:"assessmentTypeCode" dc:"Stable privacy notice category" eg:"personal_verification"`
	AssessmentNo         string `json:"assessmentNo" dc:"Published assessment number" eg:"PIA_0123456789abcdef0123456789abcdef"`
	Version              int64  `json:"version" dc:"Published notice version" eg:"1"`
	Purpose              string `json:"purpose" dc:"Why the information is processed" eg:"Verify the account owner"`
	Necessity            string `json:"necessity" dc:"Why the processing is necessary" eg:"Meet local compliance requirements"`
	Impact               string `json:"impact" dc:"Customer-facing impact description" eg:"Verification may be unavailable after withdrawal"`
	Recipients           string `json:"recipients" dc:"Who receives the minimum information" eg:"The configured local verification provider"`
	StorageLocation      string `json:"storageLocation" dc:"Where evidence is stored" eg:"This installation and the configured local provider"`
	RetentionDescription string `json:"retentionDescription" dc:"Plain-language retention explanation" eg:"Deleted after the configured period unless a legal hold applies"`
	RetentionDays        int    `json:"retentionDays" dc:"Current merchant-configured retention days" eg:"365"`
	RightsEntry          string `json:"rightsEntry" dc:"How the customer exercises privacy rights" eg:"Use Privacy and Authorization in the customer center"`
}

// CustomerPrivacyConsent is the latest decision for one consent category.
type CustomerPrivacyConsent struct {
	ConsentNo       string `json:"consentNo" dc:"Opaque consent decision number" eg:"PVC_0123456789abcdef0123456789abcdef"`
	ConsentTypeCode string `json:"consentTypeCode" dc:"Stable consent category" eg:"sensitive_personal_information"`
	ActionCode      string `json:"actionCode" dc:"Latest grant or withdrawal decision" eg:"granted"`
	DecidedAt       int64  `json:"decidedAt" dc:"Decision time as Unix milliseconds" eg:"1787047200000"`
}

// CustomerPrivacyRequest is one customer-owned export or deletion request.
type CustomerPrivacyRequest struct {
	RequestNo       string `json:"requestNo" dc:"Opaque request number" eg:"PCR_0123456789abcdef0123456789abcdef"`
	RequestTypeCode string `json:"requestTypeCode" dc:"Export or delete request type" eg:"export"`
	StatusCode      string `json:"statusCode" dc:"Current request status" eg:"submitted"`
	HoldReasonCode  string `json:"holdReasonCode" dc:"Stable reason when legal evidence must be retained" eg:""`
	SubmittedAt     int64  `json:"submittedAt" dc:"Submission time as Unix milliseconds" eg:"1787047200000"`
	CompletedAt     int64  `json:"completedAt" dc:"Completion time as Unix milliseconds; zero when incomplete" eg:"0"`
	Version         int64  `json:"version" dc:"Current optimistic version" eg:"1"`
}
