// This file defines one complete immutable drag-placement set.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplateFieldSetCreateReq struct {
	g.Meta            `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/field-sets" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Save contract template PDF fields" dc:"Append one complete immutable field set for the exact current confirmed PDF. The server revalidates the fixed variable catalog, page bounds, customer scope, signing types, parties, and normalized positions." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo        string                                 `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo         int64                                  `json:"versionNo" v:"required|min:1" dc:"Local draft version number" eg:"1"`
	ExpectedVersion   int64                                  `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"5"`
	ContentRevisionNo string                                 `json:"contentRevisionNo" v:"required|max-length:64" dc:"Exact current confirmed PDF revision" eg:"ctpc_0123456789abcdef0123456789abcdef"`
	IdempotencyKey    string                                 `json:"idempotencyKey" v:"required|length:8,128" dc:"Retry key for this exact complete field set" eg:"contract-template-fields-20260814-0001"`
	Fields            []ElectronicContractTemplateFieldInput `json:"fields" v:"required" dc:"Complete drag-placement set of one to one hundred fields; collection bounds are enforced by the template service" eg:"[]"`
}

type ElectronicContractTemplateFieldSetCreateRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Editor projection with the saved immutable field set" eg:"{}"`
}
