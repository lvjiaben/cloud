// This file defines the enterprise verification notice grant.
package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationConsentCreateReq records one explicit enterprise notice consent.
type EnterpriseVerificationConsentCreateReq struct {
	g.Meta         `path:"/account/compliance/enterprise-verification-consents" method:"post" tags:"IDC Compliance Account" summary:"Grant enterprise verification consent" dc:"Append or safely replay the authenticated enterprise customer's consent after displaying purpose, necessity, impact, recipients, storage, retention, and rights. Consent and case creation are separate writes."`
	Accepted       bool   `json:"accepted" v:"required" dc:"Explicit consent decision; only true can create a grant" eg:"true"`
	PolicyVersion  string `json:"policyVersion" v:"required|max-length:64" dc:"Displayed enterprise verification notice version" eg:"1"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Locale of the displayed notice" eg:"zh-CN"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"enterprise-consent-01J5M6S2R9Z8X7W6"`
}

// EnterpriseVerificationConsentCreateRes returns bounded consent evidence.
type EnterpriseVerificationConsentCreateRes struct {
	ConsentNo     string `json:"consentNo" dc:"Opaque consent evidence number" eg:"CNS_0123456789abcdef0123456789abcdef"`
	PolicyVersion string `json:"policyVersion" dc:"Committed notice version" eg:"1"`
	OccurredAt    int64  `json:"occurredAt" dc:"Consent time as Unix milliseconds" eg:"1786502400000"`
	Reused        bool   `json:"reused" dc:"Whether the exact grant already existed" eg:"false"`
}
