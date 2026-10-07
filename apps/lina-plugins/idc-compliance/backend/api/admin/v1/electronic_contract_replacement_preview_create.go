// This file defines the management command that prepares an immutable replacement preview.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractReplacementPreviewCreateReq prepares one changed immutable replacement.
type ElectronicContractReplacementPreviewCreateReq struct {
	g.Meta         `path:"/admin/compliance/electronic-contracts/{contractNo}/replacement-previews" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Create a replacement contract preview" dc:"Create one new immutable contract version linked to a failed or ended signing contract. This command only creates the local preview and never submits Tencent." permission:"idc:admin:compliance:contract:reissue"`
	ContractNo     string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Exact failed or ended contract being replaced" eg:"CTR_0123456789ABCDEF"`
	TemplateNo     string `json:"templateNo" v:"required|max-length:64" dc:"Exact currently enabled template selected for the new immutable preview" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	SignerNo       string `json:"signerNo" v:"max-length:64" dc:"Verified enterprise signer selected for the new preview; empty for an individual customer" eg:"csgn_0123456789abcdef0123456789abcdef"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:16,128" dc:"Retry key bound to this exact replacement preview" eg:"contract-replacement-20260817-0001"`
}

// ElectronicContractReplacementPreviewCreateRes returns only the local immutable preview.
type ElectronicContractReplacementPreviewCreateRes struct {
	Item *ElectronicContractPreparationItem `json:"item" dc:"Created or safely replayed replacement contract preview" eg:"{}"`
}
