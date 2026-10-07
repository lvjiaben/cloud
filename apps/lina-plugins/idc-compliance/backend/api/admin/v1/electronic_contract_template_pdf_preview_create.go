// This file defines explicit immutable PDF generation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplatePDFPreviewCreateReq struct {
	g.Meta            `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/pdf-previews" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Generate an immutable contract template PDF preview" dc:"Convert the exact current source through the installation-owned converter, or validate an uploaded PDF, then append encrypted source and PDF bytes as one new immutable revision. No remote conversion service is used." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo        string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo         int64  `json:"versionNo" v:"required|min:1" dc:"Local draft version number" eg:"1"`
	ExpectedVersion   int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"3"`
	ContentRevisionNo string `json:"contentRevisionNo" v:"required|max-length:64" dc:"Exact current source revision" eg:"ctpc_0123456789abcdef0123456789abcdef"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|length:8,128" dc:"Retry key for this exact conversion" eg:"contract-template-preview-20260814-0001"`
}

type ElectronicContractTemplatePDFPreviewCreateRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Editor projection with the generated immutable PDF" eg:"{}"`
}
