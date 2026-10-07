// This file defines one no-store current PDF preview response.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplatePreviewContentGetReq struct {
	g.Meta            `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/content-revisions/{contentRevisionNo}/preview" method:"get" mime:"application/pdf" tags:"IDC Compliance Electronic Contract" summary:"Open an electronic contract template PDF preview" dc:"Authorize, decrypt, authenticate, and stream only the exact current immutable PDF with no-store headers." permission:"idc:admin:compliance:contract-template:query"`
	TemplateNo        string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo         int64  `json:"versionNo" v:"required|min:1" dc:"Local version number" eg:"1"`
	ContentRevisionNo string `json:"contentRevisionNo" v:"required|max-length:64" dc:"Exact current immutable PDF revision" eg:"ctpc_0123456789abcdef0123456789abcdef"`
}

type ElectronicContractTemplatePreviewContentGetRes struct{}
