// This file defines explicit administrator confirmation of one exact PDF preview.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplatePreviewConfirmationCreateReq struct {
	g.Meta            `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/preview-confirmations" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Confirm an electronic contract template PDF preview" dc:"Record explicit administrator confirmation only for the exact current immutable PDF. Any later source or PDF revision clears this confirmation." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo        string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo         int64  `json:"versionNo" v:"required|min:1" dc:"Local draft version number" eg:"1"`
	ExpectedVersion   int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"4"`
	ContentRevisionNo string `json:"contentRevisionNo" v:"required|max-length:64" dc:"Exact current ready PDF revision" eg:"ctpc_0123456789abcdef0123456789abcdef"`
}

type ElectronicContractTemplatePreviewConfirmationCreateRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Editor projection with explicit preview confirmation" eg:"{}"`
}
