// Only manually placed identity-retention holds may be released through this endpoint.
package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyRetentionHoldReleaseCreateReq struct {
	g.Meta         `path:"/admin/compliance/privacy/retention-holds/releases" method:"post" tags:"IDC Compliance Materials" summary:"Release a manually placed material hold" dc:"Release only an identity-owned manual legal hold; tax, dispute, audit and other business-owner protections remain effective." permission:"idc:admin:compliance:privacy:configure"`
	TargetTypeCode string `json:"targetTypeCode" v:"required|in:personal_verification_case,enterprise_verification_case,enterprise_verification_material,tax_invoice_file" dc:"Retained material category" eg:"personal_verification_case"`
	TargetNo       string `json:"targetNo" v:"required|max-length:64" dc:"Opaque retained material number" eg:"PVC_0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:16,128" dc:"Random replay key" eg:"material-release-0123456789abcdef"`
}
type PrivacyRetentionHoldReleaseCreateRes struct{}
