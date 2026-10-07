package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerPhoneChallengeCompletionCreateReq struct {
	g.Meta        `path:"/account/compliance/contract-signers/{signerNo}/phone-human-challenge-completions" method:"post" tags:"IDC Compliance Account" summary:"Complete signer phone human challenge"`
	SignerNo      string `json:"signerNo" v:"required|max-length:64"`
	BrowserNonce  string `json:"browserNonce" v:"required|min-length:16|max-length:128"`
	Locale        string `json:"locale" v:"required|in:zh-CN,en-US"`
	ChallengeNo   string `json:"challengeNo" v:"required|max-length:96"`
	Answer        string `json:"answer" v:"max-length:256"`
	ProviderToken string `json:"providerToken" v:"max-length:4096"`
}
type ContractSignerPhoneChallengeCompletionCreateRes struct {
	Item ContractSignerHumanProofItem `json:"item"`
}
