// Material retention holds are managed within identity verification, never as a separate governance console.
package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyRetentionHoldCreateReq struct {
	g.Meta         `path:"/admin/compliance/privacy/retention-holds" method:"post" tags:"IDC Compliance Materials" summary:"Protect one material from deletion" dc:"Place an identity-owned manual legal hold on existing material; a later confirmation cannot remove its original until the hold is released." permission:"idc:admin:compliance:privacy:configure"`
	TargetTypeCode string `json:"targetTypeCode" v:"required|in:personal_verification_case,enterprise_verification_case,enterprise_verification_material,tax_invoice_file" dc:"Retained material category" eg:"personal_verification_case"`
	TargetNo       string `json:"targetNo" v:"required|max-length:64" dc:"Opaque retained material number" eg:"PVC_0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:16,128" dc:"Random replay key" eg:"material-hold-0123456789abcdef"`
}
type PrivacyRetentionHoldCreateRes struct{}
