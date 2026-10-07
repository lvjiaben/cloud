// This file defines first local template-draft creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateCreateReq creates one empty local draft.
type ElectronicContractTemplateCreateReq struct {
	g.Meta           `path:"/admin/compliance/electronic-contract/templates" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Create a local electronic contract template draft" dc:"Create the first metadata-only local draft. Body content is saved only by a separate immutable revision command." permission:"idc:admin:compliance:contract-template:manage"`
	Name             string `json:"name" v:"required|max-length:200" dc:"Template display name" eg:"IDC service agreement"`
	ScopeCode        string `json:"scopeCode" v:"required|max-length:16" dc:"Initial personal, enterprise, or all_customers scope" eg:"enterprise"`
	SigningOrderCode string `json:"signingOrderCode" v:"required|max-length:24" dc:"Initial customer_first, merchant_first, or parallel order" eg:"customer_first"`
	LanguageCode     string `json:"languageCode" v:"required|max-length:8" dc:"Explicit zh-CN or en-US contract language" eg:"zh-CN"`
	IdempotencyKey   string `json:"idempotencyKey" v:"required|length:8,128" dc:"Retry key for this exact draft creation" eg:"contract-template-create-20260814-0001"`
}

// ElectronicContractTemplateCreateRes returns the local draft.
type ElectronicContractTemplateCreateRes struct {
	Item *ElectronicContractTemplateItem `json:"item" dc:"Created or safely replayed local draft" eg:"{}"`
}
