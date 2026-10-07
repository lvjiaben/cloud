// This file defines the current-customer bounded refund history endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RefundListReq struct {
	g.Meta     `path:"/account/refunds" method:"get" tags:"IDC Customer Refund" summary:"List my refunds" dc:"Return one bounded newest-first page of refund requests owned by the current customer. Customer identity is never accepted from the request."`
	StatusCode string `json:"statusCode" in:"query" v:"max-length:32" dc:"Optional exact refund status filter; empty returns all statuses" eg:"awaiting_balance_consent"`
	Cursor     string `json:"cursor" in:"query" v:"max-length:40" dc:"Optional next-page cursor returned by the previous response" eg:"rfd_01J2ABCDEF"`
	Limit      int    `json:"limit" in:"query" v:"min:0|max:100" dc:"Page size from 1 to 100; zero uses 20" eg:"20"`
}

type RefundListRes struct {
	Refunds    []*Refund `json:"refunds" dc:"Newest-first current-customer refund summaries" eg:"[]"`
	NextCursor string    `json:"nextCursor" dc:"Cursor for the next page, or empty when complete" eg:"rfd_01J2ABCDEF"`
}
