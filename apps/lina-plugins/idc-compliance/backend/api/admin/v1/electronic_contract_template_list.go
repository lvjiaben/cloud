// This file defines the bounded template list API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateListReq selects one bounded page without calling Tencent.
type ElectronicContractTemplateListReq struct {
	PageSize   int `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Maximum rows per page" eg:"10"`
	g.Meta     `path:"/admin/compliance/electronic-contract/templates" method:"get" tags:"IDC Compliance Electronic Contract" summary:"List electronic contract templates" dc:"Return the latest local version of each template. Loading this list never calls Tencent." permission:"idc:admin:compliance:contract-template:query"`
	Name       string `json:"name" v:"max-length:200" dc:"Optional literal name fragment" eg:"service agreement"`
	ScopeCode  string `json:"scopeCode" v:"max-length:16" dc:"Optional personal, enterprise, or all_customers filter" eg:"enterprise"`
	StatusCode string `json:"statusCode" v:"max-length:32" dc:"Optional effective local or provider-override status" eg:"enabled"`
	Page       int    `json:"page" v:"min:0|max:10000" dc:"One-based page; zero selects page one" eg:"1"`
}

// ElectronicContractTemplateListRes returns one exact page and total.
type ElectronicContractTemplateListRes struct {
	Items    []ElectronicContractTemplateItem `json:"items" dc:"Latest logical template versions" eg:"[]"`
	Page     int                              `json:"page" dc:"One-based page" eg:"1"`
	PageSize int                              `json:"pageSize" dc:"Requested page size" eg:"20"`
	Total    int64                            `json:"total" dc:"Matching logical template count" eg:"12"`
}
