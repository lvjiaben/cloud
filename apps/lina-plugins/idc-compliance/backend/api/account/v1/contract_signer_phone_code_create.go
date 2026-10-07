package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerPhoneCodeCreateReq struct {
	g.Meta         `path:"/account/compliance/contract-signers/{signerNo}/phone-verification-codes" method:"post" tags:"IDC Compliance Account" summary:"Send signer phone verification code"`
	SignerNo       string `json:"signerNo" v:"required|max-length:64"`
	BrowserNonce   string `json:"browserNonce" v:"required|min-length:16|max-length:128"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US"`
	ProofToken     string `json:"proofToken" v:"required|max-length:4096"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:64"`
}
type ContractSignerPhoneCodeCreateRes struct {
	Item ContractSignerPhoneCodeItem `json:"item"`
}
