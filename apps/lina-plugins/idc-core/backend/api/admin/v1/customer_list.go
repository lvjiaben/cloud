package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerListReq struct {
	g.Meta       `path:"/admin/customers" method:"get" tags:"IDC Customer Management" summary:"List customers" dc:"Return one bounded keyset page of display-safe customer summaries with verification, level, and tag count projections." permission:"idc:admin:customer:list"`
	CustomerCode string `json:"customerCode" v:"regex:^$|^[0-9]{1,18}$" dc:"Optional exact short customer number" eg:"10000"`
	NamePrefix   string `json:"namePrefix" dc:"Optional literal customer name prefix; percent and underscore are not wildcards" eg:"example"`
	Phone        string `json:"phone" v:"regex:^$|^[0-9]{6,20}$" dc:"Optional exact local mobile number without calling code" eg:"13800138000"`
	Type         string `json:"type" dc:"Optional customer type filter: individual or enterprise" eg:"individual"`
	Status       string `json:"status" dc:"Optional account status filter: pending_activation, active, restricted, disabled, or anonymized" eg:"active"`
	GroupNo      string `json:"groupNo" v:"max-length:36" dc:"Optional exact active customer group business number" eg:"GRP_0123456789abcdef0123456789abcdef"`
	TagNo        string `json:"tagNo" v:"max-length:36" dc:"Optional exact active customer tag business number" eg:"TAG_0123456789abcdef0123456789abcdef"`
	Cursor       string `json:"cursor" v:"max-length:1024" dc:"Opaque cursor from the previous response; empty for the first page" eg:""`
	Page         int    `json:"page" v:"min:0" dc:"Optional one-based page number; cannot be combined with cursor" eg:"1"`
	Limit        int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum customers returned in this page" eg:"25"`
}

type CustomerListRes struct {
	Customers  []*CustomerSummaryItem `json:"customers" dc:"Display-safe customer summaries" eg:"[]"`
	NextCursor string                 `json:"nextCursor" dc:"Opaque cursor for the next page, or empty when complete" eg:""`
	Total      int                    `json:"total" dc:"Total records matching the current filters and authorized data scope" eg:"0"`
}
