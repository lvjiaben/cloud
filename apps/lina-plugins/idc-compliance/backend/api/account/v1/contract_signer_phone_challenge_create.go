package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerPhoneChallengeCreateReq struct {
	g.Meta           `path:"/account/compliance/contract-signers/{signerNo}/phone-human-challenges" method:"post" tags:"IDC Compliance Account" summary:"Create signer phone human challenge"`
	SignerNo         string `json:"signerNo" v:"required|max-length:64"`
	BrowserNonce     string `json:"browserNonce" v:"required|min-length:16|max-length:128"`
	Locale           string `json:"locale" v:"required|in:zh-CN,en-US"`
	PresentationKind string `json:"presentationKind" v:"required|in:image,accessible"`
}
type ContractSignerPhoneChallengeCreateRes struct {
	Item ContractSignerHumanChallengeItem `json:"item"`
}
