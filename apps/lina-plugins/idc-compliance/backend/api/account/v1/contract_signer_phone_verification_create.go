package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerPhoneVerificationCreateReq struct {
	g.Meta         `path:"/account/compliance/contract-signers/{signerNo}/phone-verifications" method:"post" tags:"IDC Compliance Account" summary:"Verify signer phone code"`
	SignerNo       string `json:"signerNo" v:"required|max-length:64"`
	BrowserNonce   string `json:"browserNonce" v:"required|min-length:16|max-length:128"`
	Locale         string `json:"locale" v:"required|in:zh-CN,en-US"`
	VerificationNo string `json:"verificationNo" v:"required|max-length:48"`
	Code           string `json:"code" v:"required|length:6"`
}
type ContractSignerPhoneVerificationCreateRes struct {
	Item ContractSignerItem `json:"item"`
}
