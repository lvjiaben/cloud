package v1

import "github.com/gogf/gf/v2/frame/g"

type TaxInvoiceListReq struct {
	Page       int `json:"page" v:"min:0|max:1000000" dc:"Requested page number; omit for cursor pagination" eg:"1"`
	g.Meta     `path:"/admin/compliance/tax-invoices" method:"get" tags:"IDC Compliance Tax Invoice" summary:"List tax invoice requests" dc:"Return one bounded installation-local staff page with its authorized total. Private buyer, seller, provider, and file locations are never returned." permission:"idc:admin:compliance:tax-invoice:query"`
	CustomerNo string `json:"customerNo" v:"max-length:64" dc:"Optional exact customer number" eg:"CUS_0123456789ABCDEF"`
	StatusCode string `json:"statusCode" v:"max-length:32" dc:"Optional exact request state" eg:"pending_review"`
	PageSize   int    `json:"pageSize" v:"min:0|max:100" dc:"Page size; zero selects 20" eg:"20"`
	Cursor     string `json:"cursor" v:"max-length:256" dc:"Opaque next-page cursor" eg:"eyJjcmVhdGVkQXQiOiIyMDI2LTA4LTE4VDA4OjAwOjAwWiIsImlkIjoxfQ"`
}

type TaxInvoiceListRes struct {
	Total      int                        `json:"total" dc:"Total matching records in the authorized scope" eg:"25"`
	Items      []TaxInvoiceManagementItem `json:"items" dc:"Safe request rows" eg:"[]"`
	NextCursor string                     `json:"nextCursor" dc:"Opaque next-page cursor; empty on the final page" eg:""`
}
