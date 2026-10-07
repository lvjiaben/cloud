// This file defines customer-safe personal-verification API projections.

package v1

// PersonalVerificationItem is the local or authoritatively refreshed customer projection.
type PersonalVerificationItem struct {
	CaseNo           string `json:"caseNo" dc:"Opaque personal verification case number; empty before the first attempt" eg:"case_0123456789abcdef0123456789abcdef"`
	AttemptNo        string `json:"attemptNo" dc:"Opaque provider attempt number; empty before the first attempt" eg:"pva_0123456789abcdef0123456789abcdef"`
	StatusCode       string `json:"statusCode" dc:"Customer-safe status: unverified, verifying, review_required, verified, rejected, or expired" eg:"verifying"`
	MaskedName       string `json:"maskedName" dc:"Masked legal name retained as verification evidence" eg:"T*** U***"`
	MaskedIdentityNo string `json:"maskedIdentityNo" dc:"Masked identity document number retained as verification evidence" eg:"110***********0019"`
	ContinuationURL  string `json:"continuationUrl" dc:"Fresh HTTPS Aliyun H5 continuation URL returned only by initiation" eg:"https://cloudauth.aliyuncs.com/verify/example"`
	ReasonCode       string `json:"reasonCode" dc:"Stable non-sensitive review or terminal reason code" eg:"provider_initialization_unknown"`
	SubmittedAt      int64  `json:"submittedAt" dc:"Submission time as Unix milliseconds; zero when unavailable" eg:"1786502400000"`
	VerifiedAt       int64  `json:"verifiedAt" dc:"Authoritative verification time as Unix milliseconds; zero until verified" eg:"1786502460000"`
	ExpiresAt        int64  `json:"expiresAt" dc:"Attempt expiration time as Unix milliseconds; zero when unavailable" eg:"1786504200000"`
}
