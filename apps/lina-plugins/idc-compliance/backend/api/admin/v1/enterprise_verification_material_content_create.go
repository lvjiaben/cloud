// This file defines one-time audited enterprise-material content release.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationMaterialContentCreateReq consumes one access context.
type EnterpriseVerificationMaterialContentCreateReq struct {
	g.Meta        `path:"/admin/compliance/enterprise-verifications/{caseNo}/materials/{materialNo}/content" method:"post" mime:"application/octet-stream" tags:"IDC Compliance Enterprise Review" summary:"Download one enterprise verification original" dc:"Consume one recent-login, session-bound, target-bound access context, append an immutable per-view audit event, and stream the authenticated decrypted original as a no-store attachment." permission:"idc:admin:compliance:enterprise-material:access"`
	CaseNo        string `json:"caseNo" v:"required|max-length:64" dc:"Enterprise verification case number" eg:"ent_0123456789abcdef0123456789abcdef"`
	MaterialNo    string `json:"materialNo" v:"required|max-length:64" dc:"Enterprise material number" eg:"mat_0123456789abcdef0123456789abcdef"`
	AccessContext string `json:"accessContext" v:"required|max-length:16384" dc:"Opaque access context returned by the access-context endpoint" eg:"ra1.key_01.opaque.signature"`
}

// EnterpriseVerificationMaterialContentCreateRes is an empty schema because
// the controller owns the documented binary attachment response directly.
type EnterpriseVerificationMaterialContentCreateRes struct{}
