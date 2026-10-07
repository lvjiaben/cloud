package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerDisablementCreateReq struct {
	g.Meta          `path:"/account/compliance/contract-signers/{signerNo}/disablements" method:"post" tags:"IDC Compliance Account" summary:"Disable contract signer"`
	SignerNo        string `json:"signerNo" v:"required|max-length:64"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64"`
}
type ContractSignerDisablementCreateRes struct {
	Item ContractSignerItem `json:"item"`
}
