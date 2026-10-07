// This file defines safe template copying into a new local draft.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateCopyCreateReq selects one source version.
type ElectronicContractTemplateCopyCreateReq struct {
	g.Meta         `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/copies" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Copy an electronic contract template into a local draft" dc:"Copy safe metadata into a new local draft. Tencent-owned or previously used body bytes and provider identifiers are never copied." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo     string `json:"templateNo" v:"required|max-length:64" dc:"Source local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo      int64  `json:"versionNo" v:"required|min:1" dc:"Source local version number" eg:"1"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,128" dc:"Retry key for this exact copy" eg:"contract-template-copy-20260814-0001"`
}

// ElectronicContractTemplateCopyCreateRes returns the new local draft.
type ElectronicContractTemplateCopyCreateRes struct {
	Item *ElectronicContractTemplateItem `json:"item" dc:"New local metadata-only draft" eg:"{}"`
}
