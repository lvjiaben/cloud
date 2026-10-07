package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractAutoSignAuthorizationListReq selects one bounded history page.
type ElectronicContractAutoSignAuthorizationListReq struct {
	PageSize int `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Maximum rows per page" eg:"10"`
	g.Meta   `path:"/admin/compliance/electronic-contract/auto-sign-authorizations" method:"get" tags:"IDC Compliance Electronic Contract" summary:"List enterprise auto-sign authorizations" dc:"Return one bounded installation-wide authorization history page and up to fifty currently eligible exact template versions without calling Tencent." permission:"idc:admin:compliance:contract-auto-sign:manage"`
	Page     int `json:"page" v:"min:0|max:10000" dc:"One-based page; zero selects page one" eg:"1"`
}

// ElectronicContractAutoSignAuthorizationListRes includes history and bounded candidates.
type ElectronicContractAutoSignAuthorizationListRes struct {
	Items      []ElectronicContractAutoSignAuthorizationItem `json:"items" dc:"Immutable authorization history" eg:"[]"`
	Candidates []ElectronicContractAutoSignTemplateRef       `json:"candidates" dc:"Currently enabled enterprise-compatible exact template versions" eg:"[]"`
	Page       int                                           `json:"page" dc:"One-based page" eg:"1"`
	PageSize   int                                           `json:"pageSize" dc:"Requested page size" eg:"20"`
	Total      int64                                         `json:"total" dc:"Total authorization history rows" eg:"3"`
}
