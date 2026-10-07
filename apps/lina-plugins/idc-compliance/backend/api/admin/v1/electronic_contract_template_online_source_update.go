// This file defines sanitized online-editor source replacement.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplateOnlineSourceUpdateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/online-source" method:"put" tags:"IDC Compliance Electronic Contract" summary:"Save online contract template source" dc:"Sanitize and append one immutable HTML source revision. Arbitrary images, scripts, unsafe links, styles, and custom variables are rejected." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo      string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo       int64  `json:"versionNo" v:"required|min:1" dc:"Local draft version number" eg:"1"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|length:8,128" dc:"Retry key for this exact HTML body" eg:"contract-template-online-source-20260814-0001"`
	HTML            string `json:"html" v:"required|max-length:5242880" dc:"Online editor HTML using only the documented safe formatting and fixed variables" eg:"<h1>Service agreement</h1><p>{{customer_name}}</p>"`
}

type ElectronicContractTemplateOnlineSourceUpdateRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Saved current editor projection" eg:"{}"`
}
