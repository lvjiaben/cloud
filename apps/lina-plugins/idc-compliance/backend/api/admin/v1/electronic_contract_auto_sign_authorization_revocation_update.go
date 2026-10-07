package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractAutoSignAuthorizationRevocationUpdateReq immediately revokes one active authorization.
type ElectronicContractAutoSignAuthorizationRevocationUpdateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contract/auto-sign-authorizations/{authorizationNo}/revocation" method:"put" tags:"IDC Compliance Electronic Contract" summary:"Revoke enterprise auto-sign authorization" dc:"Immediately stop new auto-sign starts for one active authorization after one explicit confirmation; existing and manual signing flows remain unchanged." permission:"idc:admin:compliance:contract-auto-sign:manage"`
	AuthorizationNo string `json:"authorizationNo" v:"required|max-length:64" dc:"Stable authorization number from the route" eg:"casa_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Current optimistic lock version" eg:"1"`
	Confirmed       bool   `json:"confirmed" v:"required|in:true" dc:"True only after the administrator confirms immediate revocation" eg:"true"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|length:8,128" dc:"Stable retry key; reusing it with changed input is rejected" eg:"contract-auto-sign-revoke-20260816-001"`
}

type ElectronicContractAutoSignAuthorizationRevocationUpdateRes struct {
	Item ElectronicContractAutoSignAuthorizationItem `json:"item" dc:"Revoked or safely replayed authorization" eg:"{}"`
}
