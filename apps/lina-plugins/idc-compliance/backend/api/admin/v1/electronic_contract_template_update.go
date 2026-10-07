// This file defines manual draft save and synchronized local-setting completion.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateUpdateReq saves mutable metadata with optimistic concurrency.
type ElectronicContractTemplateUpdateReq struct {
	g.Meta           `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}" method:"put" tags:"IDC Compliance Electronic Contract" summary:"Save electronic contract template draft metadata" dc:"Save only mutable local draft metadata. For Tencent templates, the provider-owned name remains read-only and this command only completes local scope and signing order." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo       string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo        int64  `json:"versionNo" v:"required|min:1" dc:"Local version number" eg:"1"`
	Name             string `json:"name" v:"required|max-length:200" dc:"Template name; Tencent-synchronized names cannot be changed here" eg:"IDC service agreement"`
	ScopeCode        string `json:"scopeCode" v:"required|max-length:16" dc:"personal, enterprise, or all_customers" eg:"enterprise"`
	SigningOrderCode string `json:"signingOrderCode" v:"required|max-length:24" dc:"customer_first, merchant_first, or parallel" eg:"customer_first"`
	LanguageCode     string `json:"languageCode" v:"required|max-length:8" dc:"Explicit zh-CN or en-US contract language" eg:"zh-CN"`
	ExpectedVersion  int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"1"`
}

// ElectronicContractTemplateUpdateRes returns the saved projection.
type ElectronicContractTemplateUpdateRes struct {
	Item *ElectronicContractTemplateItem `json:"item" dc:"Saved template projection" eg:"{}"`
}
