// This file defines explicit local template enablement.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateEnablementCreateReq enables one exact local version.
type ElectronicContractTemplateEnablementCreateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/enablements" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Enable an electronic contract template version" dc:"Enable one fully configured version and atomically disable any prior enabled version of the same logical template." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo      string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo       int64  `json:"versionNo" v:"required|min:1" dc:"Local version number" eg:"2"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"3"`
}

// ElectronicContractTemplateEnablementCreateRes returns the enabled version.
type ElectronicContractTemplateEnablementCreateRes struct {
	Item *ElectronicContractTemplateItem `json:"item" dc:"Enabled local version" eg:"{}"`
}
