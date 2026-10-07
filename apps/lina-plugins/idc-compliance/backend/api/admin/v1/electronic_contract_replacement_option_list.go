// This file defines safe template and signer choices for a changed contract reissue.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractReplacementOptionListReq reads current replacement choices.
type ElectronicContractReplacementOptionListReq struct {
	g.Meta     `path:"/admin/compliance/electronic-contracts/{contractNo}/replacement-options" method:"get" tags:"IDC Compliance Electronic Contract" summary:"List replacement contract options" dc:"Return currently enabled templates and verified enterprise signing handlers for one reissuable contract. Loading choices never creates a preview or contacts Tencent." permission:"idc:admin:compliance:contract:reissue"`
	ContractNo string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Exact rejected, expired, canceled, or failed contract being replaced" eg:"CTR_0123456789ABCDEF"`
}

// ElectronicContractReplacementOptionListRes returns safe current choices.
type ElectronicContractReplacementOptionListRes struct {
	ContractNo   string                                    `json:"contractNo" dc:"Reissuable old contract number" eg:"CTR_0123456789ABCDEF"`
	OrderNo      string                                    `json:"orderNo" dc:"Opaque order number preserved by the replacement" eg:"ORDER_20260815-003"`
	CustomerNo   string                                    `json:"customerNo" dc:"Opaque customer owner number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	CustomerType string                                    `json:"customerType" dc:"Verified customer type: individual or enterprise" eg:"enterprise"`
	Templates    []ElectronicContractTemplateCandidateItem `json:"templates" dc:"Currently enabled templates matching the exact customer type" eg:"[]"`
	Signers      []ElectronicContractReplacementSignerItem `json:"signers" dc:"Verified active enterprise handlers; empty for an individual customer" eg:"[]"`
}

// ElectronicContractReplacementSignerItem is a masked verified handler choice.
type ElectronicContractReplacementSignerItem struct {
	SignerNo    string `json:"signerNo" dc:"Opaque verified handler number" eg:"csgn_0123456789abcdef0123456789abcdef"`
	MaskedName  string `json:"maskedName" dc:"Masked verified handler name" eg:"Z*"`
	MaskedPhone string `json:"maskedPhone" dc:"Masked verified handler phone" eg:"138****8000"`
}
