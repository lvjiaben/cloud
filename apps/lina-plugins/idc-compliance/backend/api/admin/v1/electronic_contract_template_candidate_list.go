// This file defines order-scoped optional-contract template candidates.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateCandidateListReq lists templates usable by one order.
type ElectronicContractTemplateCandidateListReq struct {
	g.Meta  `path:"/admin/compliance/electronic-contracts/template-candidates" method:"get" tags:"IDC Compliance Electronic Contract" summary:"List optional contract template candidates" dc:"Return only enabled templates matching the exact optional-contract order and its verified customer type. The independent contract-create permission is sufficient." permission:"idc:admin:compliance:contract:create"`
	OrderNo string `json:"orderNo" in:"query" v:"required|max-length:40" dc:"Exact order number inside the caller's evaluated data scope" eg:"ORDER_20260815-003"`
}

// ElectronicContractTemplateCandidateListRes returns bounded safe metadata only.
type ElectronicContractTemplateCandidateListRes struct {
	OrderNo      string                                    `json:"orderNo" dc:"Exact authorized order number" eg:"ORDER_20260815-003"`
	CustomerNo   string                                    `json:"customerNo" dc:"Opaque customer owner number used to load verified enterprise signers; empty when the order already has a contract" eg:"CUS_0123456789abcdef0123456789abcdef"`
	CustomerType string                                    `json:"customerType" dc:"Verified order customer type: individual or enterprise; empty when the order already has a contract" eg:"enterprise"`
	Items        []ElectronicContractTemplateCandidateItem `json:"items" dc:"Enabled templates usable by this exact order" eg:"[]"`
}

// ElectronicContractTemplateCandidateItem is one safe template choice.
type ElectronicContractTemplateCandidateItem struct {
	TemplateNo       string `json:"templateNo" dc:"Installation-local opaque template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	TemplateVersion  int64  `json:"templateVersion" dc:"Exact immutable enabled version" eg:"5"`
	TemplateName     string `json:"templateName" dc:"Template display name" eg:"Optional service contract"`
	ScopeCode        string `json:"scopeCode" dc:"personal, enterprise, or all_customers" eg:"all_customers"`
	SigningOrderCode string `json:"signingOrderCode" dc:"customer_first, merchant_first, or parallel" eg:"customer_first"`
	LanguageCode     string `json:"languageCode" dc:"Explicit zh-CN or en-US contract language" eg:"en-US"`
}
