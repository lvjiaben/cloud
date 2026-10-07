// This file defines the safe redirect to a short-lived Tencent template preview.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplateProviderPreviewGetReq struct {
	g.Meta     `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/provider-preview" method:"get" tags:"IDC Compliance Electronic Contract" summary:"Open a synchronized electronic contract template preview" dc:"Authorize the local template, resolve its server-owned Tencent reference, and redirect to one official short-lived HTTPS preview." permission:"idc:admin:compliance:contract-template:query"`
	TemplateNo string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo  int64  `json:"versionNo" v:"required|min:1" dc:"Local version number" eg:"1"`
}

type ElectronicContractTemplateProviderPreviewGetRes struct {
	PreviewURL string `json:"previewUrl" dc:"Official short-lived Tencent HTTPS preview URL" eg:"https://qian.tencent.com/preview/example"`
}
