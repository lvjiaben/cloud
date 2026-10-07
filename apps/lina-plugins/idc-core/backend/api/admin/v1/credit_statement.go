// This file defines credit statement list and detail management contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ManagementCreditStatementListReq lists bounded credit statements for staff.
type ManagementCreditStatementListReq struct {
	Page         int `json:"page" v:"min:0" dc:"Optional one-based page number; omit or zero for cursor paging; cannot combine with cursor" eg:"1"`
	g.Meta       `path:"/admin/billing/credit/statements" method:"get" tags:"IDC Credit Management" summary:"List credit statements" dc:"Return one bounded page and exact filtered total after exact statement-list permission." permission:"idc:admin:billing:credit-statement:list"`
	CustomerNo   string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number" eg:"CUS_01J2ABCDEF"`
	StatusCode   string `json:"statusCode" dc:"Optional exact statement status: open, partially_paid, paid, overdue, written_off, cancelled, or refunded" eg:"overdue"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum statements returned in this page" eg:"20"`
}

// ManagementCreditStatementListRes returns one bounded statement page.
type ManagementCreditStatementListRes struct {
	Total      int                          `json:"total" dc:"Exact authorized record count matching the current filters before pagination" eg:"123"`
	Statements []*ManagementCreditStatement `json:"statements" dc:"Bounded authorized credit statements" eg:"[]"`
	NextCursor string                       `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

// ManagementCreditStatementDetailReq requests one statement and a bounded line page.
type ManagementCreditStatementDetailReq struct {
	g.Meta      `path:"/admin/billing/credit/statements/{statementNo}" method:"get" tags:"IDC Credit Management" summary:"Get credit statement detail" dc:"Return one statement, a bounded immutable line page, and safe repayment summaries after exact detail permission." permission:"idc:admin:billing:credit-statement:query"`
	StatementNo string `json:"statementNo" in:"path" v:"required|max-length:40" dc:"Stable credit statement number" eg:"CST_01J2ABCDEF"`
	LineCursor  string `json:"lineCursor" v:"max-length:128" dc:"Optional encrypted statement-line cursor" eg:"AQIDBA"`
	LineLimit   int    `json:"lineLimit" d:"50" v:"min:1|max:100" dc:"Maximum immutable lines returned in this page" eg:"50"`
}

// ManagementCreditStatementDetailRes returns one authorized statement aggregate.
type ManagementCreditStatementDetailRes struct {
	Detail *ManagementCreditStatementDetail `json:"detail" dc:"Authorized credit statement aggregate" eg:"{}"`
}
