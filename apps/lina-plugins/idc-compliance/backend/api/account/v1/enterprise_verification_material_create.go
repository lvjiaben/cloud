// This file defines enterprise material upload reservation creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationMaterialCreateReq reserves a server-assigned material
// number before the original document enters the raw content endpoint.
type EnterpriseVerificationMaterialCreateReq struct {
	g.Meta           `path:"/account/compliance/enterprise-verifications/{caseNo}/materials" method:"post" tags:"IDC Compliance Account" summary:"Reserve enterprise verification material upload" dc:"Create an idempotent, server-assigned upload reservation for one bounded enterprise evidence category. The original document is sent separately and no storage path, digest, filename, or access URL is returned."`
	CaseNo           string `json:"caseNo" v:"required|max-length:64" dc:"Owned enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	MaterialTypeCode string `json:"materialTypeCode" v:"required|max-length:40" dc:"Evidence category: business_license, legal_representative_id, agent_id, authorization_letter, organization_certificate, or other" eg:"business_license"`
	IdempotencyKey   string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Retry key used only by the reservation command and never persisted verbatim" eg:"enterprise-material-01J5M6S2R9Z8X7W6"`
}

// EnterpriseVerificationMaterialCreateRes returns a safe upload reservation.
type EnterpriseVerificationMaterialCreateRes struct {
	Item *EnterpriseVerificationMaterialReservationItem `json:"item" dc:"Upload reservation without storage or document metadata" eg:"{}"`
}
