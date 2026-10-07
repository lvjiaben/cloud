// This file defines enterprise verification draft creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationCreateReq opens one customer-owned draft.
type EnterpriseVerificationCreateReq struct {
	g.Meta               `path:"/account/compliance/enterprise-verifications" method:"post" tags:"IDC Compliance Account" summary:"Create enterprise verification draft" dc:"Create one encrypted-material enterprise verification draft for the authenticated enterprise customer. For legal profile changes, legalChangeRequestNo references the category-only core request."`
	ConsentNo            string `json:"consentNo" v:"required|max-length:64" dc:"Latest effective enterprise verification consent number" eg:"CNS_0123456789abcdef0123456789abcdef"`
	LegalChangeRequestNo string `json:"legalChangeRequestNo" v:"max-length:40" dc:"Optional opaque core legal-change request number for re-verification" eg:"LCR_0123456789abcdef0123456789abcdef"`
	IdempotencyKey       string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"enterprise-draft-01J5M6S2R9Z8X7W6"`
}

// EnterpriseVerificationCreateRes returns the safe draft projection.
type EnterpriseVerificationCreateRes struct {
	Item *EnterpriseVerificationDraftItem `json:"item" dc:"Customer-safe enterprise verification draft" eg:"{}"`
}
