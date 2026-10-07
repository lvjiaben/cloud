// This file defines raw content upload for a reserved enterprise material.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationMaterialContentUpdateReq streams one original directly
// to the controller. The request is not multipart, so no browser filename or
// framework-managed plaintext temporary file enters the application.
type EnterpriseVerificationMaterialContentUpdateReq struct {
	g.Meta     `path:"/account/compliance/enterprise-verifications/{caseNo}/materials/{materialNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Compliance Account" summary:"Upload reserved enterprise verification material content" dc:"Stream one PDF, JPEG, or PNG original for a server-assigned reservation. The compliance service content-sniffs, encrypts, stores privately, and confirms the ciphertext before returning safe metadata."`
	CaseNo     string `json:"caseNo" v:"required|max-length:64" dc:"Owned enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	MaterialNo string `json:"materialNo" v:"required|max-length:64" dc:"Server-assigned material reservation number" eg:"mat_0123456789abcdef0123456789abcdef"`
}

// EnterpriseVerificationMaterialContentUpdateRes returns confirmed safe metadata.
type EnterpriseVerificationMaterialContentUpdateRes struct {
	Item *EnterpriseVerificationMaterialItem `json:"item" dc:"Confirmed encrypted material metadata without a private object reference" eg:"{}"`
}
