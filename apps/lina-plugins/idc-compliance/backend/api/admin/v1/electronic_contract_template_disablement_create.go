// This file defines explicit local template disablement.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateDisablementCreateReq disables one exact local version.
type ElectronicContractTemplateDisablementCreateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/disablements" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Disable an electronic contract template version" dc:"Prevent new contracts from selecting this version while retaining all immutable history." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo      string `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo       int64  `json:"versionNo" v:"required|min:1" dc:"Local version number" eg:"2"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"4"`
}

// ElectronicContractTemplateDisablementCreateRes returns the disabled version.
type ElectronicContractTemplateDisablementCreateRes struct {
	Item *ElectronicContractTemplateItem `json:"item" dc:"Disabled local version" eg:"{}"`
}
