// This file defines the exact template-version detail API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateGetReq reads one exact local immutable version projection.
type ElectronicContractTemplateGetReq struct {
	g.Meta     `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}" method:"get" tags:"IDC Compliance Electronic Contract" summary:"Get an electronic contract template version" dc:"Return one exact safe local template version without supplier identifiers or body storage references." permission:"idc:admin:compliance:contract-template:query"`
	TemplateNo string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo  int64  `json:"versionNo" v:"required|min:1" dc:"Exact local version number" eg:"1"`
}

// ElectronicContractTemplateGetRes wraps one exact version.
type ElectronicContractTemplateGetRes struct {
	Item *ElectronicContractTemplateItem `json:"item" dc:"Exact safe template version" eg:"{}"`
}
