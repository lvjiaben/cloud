// This file defines one-time private enterprise-material access issuance.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationMaterialAccessCreateReq issues a current-session-bound context.
type EnterpriseVerificationMaterialAccessCreateReq struct {
	g.Meta     `path:"/admin/compliance/enterprise-verifications/{caseNo}/materials/{materialNo}/access-contexts" method:"post" tags:"IDC Compliance Enterprise Review" summary:"Create one-time enterprise material access" dc:"After exact material-access authorization and a current active session, issue a short-lived, session-bound, target-bound, single-use context. No permanent URL is created." permission:"idc:admin:compliance:enterprise-material:access"`
	CaseNo     string `json:"caseNo" v:"required|max-length:64" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	MaterialNo string `json:"materialNo" v:"required|max-length:64" dc:"Enterprise material number" eg:"mat_0123456789abcdef0123456789abcdef"`
}

// EnterpriseVerificationMaterialAccessCreateRes returns an opaque one-time context.
type EnterpriseVerificationMaterialAccessCreateRes struct {
	AccessContext string `json:"accessContext" dc:"Opaque, signed, session-bound, one-time access context" eg:"ra1.key_01.opaque.signature"`
	ExpiresAt     int64  `json:"expiresAt" dc:"Hard access-context expiry Unix timestamp in milliseconds" eg:"1786464120000"`
}
