// This file defines the masked enterprise-verification case detail API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationCaseGetReq reads one masked case detail.
type EnterpriseVerificationCaseGetReq struct {
	g.Meta `path:"/admin/compliance/enterprise-verifications/{caseNo}" method:"get" tags:"IDC Compliance Enterprise Review" summary:"Get an enterprise verification case" dc:"Return one masked legal profile and one bounded material metadata batch. Original bytes require the separate current-session one-time access flow." permission:"idc:admin:compliance:enterprise-verification:query"`
	CaseNo string `json:"caseNo" v:"required|max-length:64" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
}

// EnterpriseVerificationCaseGetRes wraps one masked case detail.
type EnterpriseVerificationCaseGetRes struct {
	Item *EnterpriseVerificationCaseDetail `json:"item" dc:"Masked enterprise verification case detail" eg:"{}"`
}

// EnterpriseVerificationCaseDetail contains no original or permanent access URL.
type EnterpriseVerificationCaseDetail struct {
	EnterpriseVerificationCaseItem
	LegalProfile *EnterpriseVerificationMaskedLegalProfile `json:"legalProfile" dc:"Masked legal profile read from the authenticated private snapshot" eg:"{}"`
	Materials    []EnterpriseVerificationMaterialItem      `json:"materials" dc:"Bounded material metadata without private references" eg:"[]"`
	VerifiedAt   *int64                                    `json:"verifiedAt" dc:"Verification Unix timestamp in milliseconds; null unless approved" eg:"1786464060000"`
	RejectedAt   *int64                                    `json:"rejectedAt" dc:"Rejection Unix timestamp in milliseconds; null unless rejected" eg:"1786464060000"`
	ExpiresAt    *int64                                    `json:"expiresAt" dc:"Verification expiry Unix timestamp in milliseconds; null when not configured" eg:"1818000000000"`
}
