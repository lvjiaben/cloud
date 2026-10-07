// This file defines the bounded enterprise-verification management directory API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationCaseListReq reads one bounded page with its authorized total.
type EnterpriseVerificationCaseListReq struct {
	Page       int `json:"page" v:"min:0|max:1000000" dc:"Requested page number; omit for cursor pagination" eg:"1"`
	g.Meta     `path:"/admin/compliance/enterprise-verifications" method:"get" tags:"IDC Compliance Enterprise Review" summary:"List enterprise verification cases" dc:"Return one bounded page and authorized total of submitted enterprise verification cases without raw legal values, original materials, storage references, or unmasked values." permission:"idc:admin:compliance:enterprise-verification:list"`
	StatusCode string `json:"statusCode" v:"max-length:24" dc:"Optional exact status filter: pending, verifying, review_required, verified, rejected, expired, or canceled; empty includes all submitted cases" eg:"pending"`
	CustomerNo string `json:"customerNo" v:"max-length:64" dc:"Optional exact opaque customer number filter" eg:"CUS_0123456789abcdef0123456789abcdef"`
	PageSize   int    `json:"pageSize" v:"min:0|max:100" dc:"Page size; zero selects 20 and the maximum is 100" eg:"20"`
	Cursor     string `json:"cursor" v:"max-length:2048" dc:"Opaque next-page cursor from the preceding response" eg:"ec1.key_01.eyJ2IjoxfQ.signature"`
}

// EnterpriseVerificationCaseListRes wraps a bounded page with its authorized total.
type EnterpriseVerificationCaseListRes struct {
	Total      int                              `json:"total" dc:"Total matching records in the authorized scope" eg:"25"`
	Items      []EnterpriseVerificationCaseItem `json:"items" dc:"Submitted enterprise verification cases" eg:"[]"`
	NextCursor string                           `json:"nextCursor" dc:"Opaque next-page cursor; empty on the final page" eg:"ec1.key_01.eyJ2IjoxfQ.signature"`
}
