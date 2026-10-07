package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerIdentityCreateReq struct {
	g.Meta   `path:"/account/compliance/contract-signers/{signerNo}/identity-verifications" method:"post" tags:"IDC Compliance Account" summary:"Start contract signer identity verification"`
	SignerNo string `json:"signerNo" v:"required|max-length:64"`
}
type ContractSignerIdentityCreateRes struct {
	Item ContractSignerIdentityStartItem `json:"item"`
}
