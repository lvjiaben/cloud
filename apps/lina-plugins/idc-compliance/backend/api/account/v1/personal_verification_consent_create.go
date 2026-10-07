// This file defines a separate sensitive-personal-information consent grant.

package v1

import "github.com/gogf/gf/v2/frame/g"

// PersonalVerificationConsentCreateReq records one explicit separate consent.
type PersonalVerificationConsentCreateReq struct {
	g.Meta         `path:"/account/compliance/personal-verification-consents" method:"post" tags:"IDC Compliance Account" summary:"Grant separate personal verification consent" dc:"Append or safely replay the authenticated customer's separate sensitive-personal-information consent. Consent recording and verification initiation remain distinct writes."`
	Accepted       bool   `json:"accepted" v:"required" dc:"Explicit separate consent decision; only true can create a grant" eg:"true"`
	PolicyVersion  string `json:"policyVersion" v:"required|max-length:64" dc:"Policy version displayed and accepted by the customer" eg:"1"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Locale of the displayed consent notice" eg:"zh-CN"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"personal-consent-01J5M6S2R9Z8X7W6"`
}

// PersonalVerificationConsentCreateRes returns bounded consent evidence.
type PersonalVerificationConsentCreateRes struct {
	ConsentNo     string `json:"consentNo" dc:"Opaque consent evidence number" eg:"CNS_0123456789abcdef0123456789abcdef"`
	PolicyVersion string `json:"policyVersion" dc:"Committed policy version" eg:"1"`
	OccurredAt    int64  `json:"occurredAt" dc:"Consent decision time as Unix milliseconds" eg:"1786502400000"`
	Reused        bool   `json:"reused" dc:"Whether the exact idempotent grant already existed" eg:"false"`
}
