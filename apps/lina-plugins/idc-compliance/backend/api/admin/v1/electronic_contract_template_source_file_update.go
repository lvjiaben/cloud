// This file defines raw uploaded-source replacement for one local template draft.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplateSourceFileUpdateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/source-file" method:"put" mime:"application/octet-stream" tags:"IDC Compliance Electronic Contract" summary:"Save an uploaded contract template source" dc:"Stream one DOC, DOCX, PDF, or HTML source. The service verifies its content, encrypts it in private plugin storage, appends an immutable revision, and clears stale preview confirmation and fields." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo      string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo       int64  `json:"versionNo" v:"required|min:1" dc:"Local draft version number" eg:"1"`
	ExpectedVersion int64  `json:"expectedVersion" in:"query" v:"required|min:1" dc:"Last observed lock version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" in:"query" v:"required|length:8,128" dc:"Retry key for this exact source body" eg:"contract-template-source-20260814-0001"`
}

type ElectronicContractTemplateSourceFileUpdateRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Saved current editor projection" eg:"{}"`
}
