package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerIdentityCheckCreateReq struct {
	g.Meta    `path:"/account/compliance/contract-signers/{signerNo}/identity-verifications/{attemptNo}/authoritative-checks" method:"post" tags:"IDC Compliance Account" summary:"Recheck contract signer identity verification"`
	SignerNo  string `json:"signerNo" v:"required|max-length:64"`
	AttemptNo string `json:"attemptNo" v:"required|max-length:64"`
}
type ContractSignerIdentityCheckCreateRes struct {
	Item ContractSignerItem `json:"item"`
}
