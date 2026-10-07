package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerCreateReq struct {
	g.Meta               `path:"/account/compliance/contract-signers" method:"post" tags:"IDC Compliance Account" summary:"Add enterprise contract signer" dc:"Encrypt and add one signer to the authenticated enterprise's independent directory. Notification contacts are never reused."`
	CertType             string `json:"certType" v:"required|in:IDENTITY_CARD"`
	CertName             string `json:"certName" v:"required|max-length:64"`
	CertNo               string `json:"certNo" v:"required|max-length:64"`
	Phone                string `json:"phone" v:"required|max-length:16"`
	ConsentAccepted      bool   `json:"consentAccepted" v:"required"`
	ConsentPolicyVersion string `json:"consentPolicyVersion" v:"required|in:1"`
	IdempotencyKey       string `json:"idempotencyKey" v:"required|min-length:8|max-length:64"`
}
type ContractSignerCreateRes struct {
	Item ContractSignerItem `json:"item"`
}
