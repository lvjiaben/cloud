package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractSignerCandidateListReq struct {
	g.Meta     `path:"/admin/compliance/customers/{customerNo}/contract-signer-candidates" method:"get" tags:"IDC Compliance Electronic Contract" summary:"List verified enterprise contract signer candidates" dc:"Return at most 100 active enterprise signing handlers whose identity and phone are both verified for the exact customer. The response contains only masked display fields and local opaque references." permission:"idc:admin:compliance:contract:create"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:64" dc:"Exact enterprise customer owner number returned by the eligible order lookup" eg:"CUS_0123456789abcdef0123456789abcdef"`
}

type ElectronicContractSignerCandidateListRes struct {
	Items []ElectronicContractSignerCandidateItem `json:"items" dc:"Bounded verified signer candidates in stable masked-name order" eg:"[]"`
}

type ElectronicContractSignerCandidateItem struct {
	SignerNo                    string `json:"signerNo" dc:"Installation-local opaque signer number" eg:"csgn_0123456789abcdef0123456789abcdef"`
	MaskedName                  string `json:"maskedName" dc:"Masked verified signer name" eg:"Z*"`
	MaskedPhone                 string `json:"maskedPhone" dc:"Masked verified mobile number" eg:"138****8000"`
	IdentityVerificationVersion int64  `json:"identityVerificationVersion" dc:"Frozen identity-verification revision" eg:"3"`
	IdentityVerifiedAt          int64  `json:"identityVerifiedAt" dc:"Identity verification time as Unix timestamp in milliseconds" eg:"1787040000000"`
	PhoneVerificationVersion    int64  `json:"phoneVerificationVersion" dc:"Frozen phone-verification revision" eg:"4"`
	PhoneVerifiedAt             int64  `json:"phoneVerifiedAt" dc:"Phone verification time as Unix timestamp in milliseconds" eg:"1787040000000"`
	AuthorizationVersion        int64  `json:"authorizationVersion" dc:"Enterprise signing authorization revision" eg:"2"`
	Version                     int64  `json:"version" dc:"Current optimistic signer version" eg:"5"`
}
