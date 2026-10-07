// This file defines confirmation of one complete Tencent field and signer mapping.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractTemplateProviderFieldMappingInput struct {
	ComponentRef string `json:"componentRef" v:"required|max-length:128" dc:"Exact opaque Tencent position reference from the editor projection" eg:"component-1"`
	ValueCode    string `json:"valueCode" v:"required|max-length:32" dc:"One fixed contract value code" eg:"service_lines"`
}

type ElectronicContractTemplateProviderRecipientMappingInput struct {
	RecipientRef string `json:"recipientRef" v:"required|max-length:128" dc:"Exact opaque Tencent signer-role reference from the editor projection" eg:"recipient-1"`
	PartyCode    string `json:"partyCode" v:"required|in:customer,merchant" dc:"customer or merchant" eg:"customer"`
}

type ElectronicContractTemplateProviderMappingCreateReq struct {
	g.Meta            `path:"/admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/provider-mappings" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Confirm Tencent template field and signer mappings" dc:"Confirm one complete immutable mapping for the exact synchronized Tencent template version. Required positions, the service details table, both signing parties, customer type, and signing order are revalidated by the server." permission:"idc:admin:compliance:contract-template:manage"`
	TemplateNo        string                                                    `json:"templateNo" v:"required|max-length:64" dc:"Installation-local template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo         int64                                                     `json:"versionNo" v:"required|min:1" dc:"Exact synchronized template version" eg:"1"`
	ExpectedVersion   int64                                                     `json:"expectedVersion" v:"required|min:1" dc:"Last observed lock version" eg:"2"`
	IdempotencyKey    string                                                    `json:"idempotencyKey" v:"required|length:8,128" dc:"Retry key for this exact confirmation" eg:"provider-mapping-20260816-0001"`
	FieldMappings     []ElectronicContractTemplateProviderFieldMappingInput     `json:"fieldMappings" v:"required" dc:"One to five hundred twelve explicit provider position mappings; collection bounds are enforced by the mapping service" eg:"[]"`
	RecipientMappings []ElectronicContractTemplateProviderRecipientMappingInput `json:"recipientMappings" v:"required" dc:"Two to sixty-four explicit signer-role mappings; collection bounds are enforced by the mapping service" eg:"[]"`
}

type ElectronicContractTemplateProviderMappingCreateRes struct {
	Item *ElectronicContractTemplateEditorItem `json:"item" dc:"Editor projection with the confirmed immutable mapping" eg:"{}"`
}
