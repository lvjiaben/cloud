// This file defines the exact current contract-template editor projection.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplateEditorGetReq struct {
	g.Meta     `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/editor" method:"get" tags:"IDC Compliance Electronic Contract" summary:"Get the electronic contract template editor" dc:"Return the exact current immutable content and field projection. Tencent list or content APIs are never called implicitly." permission:"idc:admin:compliance:contract-template:query"`
	TemplateNo string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo  int64  `json:"versionNo" v:"required|min:1" dc:"Exact local version number" eg:"1"`
}

type ElectronicContractTemplateEditorGetRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Current safe editor projection" eg:"{}"`
}
