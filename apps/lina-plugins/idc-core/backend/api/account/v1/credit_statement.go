// This file defines customer credit statement list and detail contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CreditStatementListReq lists the current customer's bounded credit statements.
type CreditStatementListReq struct {
	Page         int `json:"page" v:"min:0" dc:"Optional one-based page number; omit or zero for cursor paging; cannot combine with cursor" eg:"1"`
	g.Meta       `path:"/account/billing/credit/statements" method:"get" tags:"IDC Customer Credit" summary:"List my credit statements" dc:"Return one bounded page and exact filtered total with current same-currency balance repayment eligibility."`
	StatusCode   string `json:"statusCode" dc:"Optional exact statement status: open, partially_paid, paid, overdue, written_off, cancelled, or refunded" eg:"overdue"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum statements returned in this page" eg:"20"`
}

// CreditStatementListRes returns one bounded statement page.
type CreditStatementListRes struct {
	Total      int                `json:"total" dc:"Exact authorized record count matching the current filters before pagination" eg:"123"`
	Statements []*CreditStatement `json:"statements" dc:"Bounded current-customer credit statements" eg:"[]"`
	NextCursor string             `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

// CreditStatementDetailReq requests one owned statement and a bounded line page.
type CreditStatementDetailReq struct {
	g.Meta      `path:"/account/billing/credit/statements/{statementNo}" method:"get" tags:"IDC Customer Credit" summary:"Get my credit statement" dc:"Return one owned statement, a bounded immutable line page, and safe repayment summaries."`
	StatementNo string `json:"statementNo" in:"path" v:"required|max-length:40" dc:"Stable current-customer credit statement number" eg:"CST_01J2ABCDEF"`
	LineCursor  string `json:"lineCursor" v:"max-length:128" dc:"Optional encrypted statement-line cursor" eg:"AQIDBA"`
	LineLimit   int    `json:"lineLimit" d:"50" v:"min:1|max:100" dc:"Maximum immutable lines returned in this page" eg:"50"`
}

// CreditStatementDetailRes returns one bounded customer-safe statement aggregate.
type CreditStatementDetailRes struct {
	Detail *CreditStatementDetail `json:"detail" dc:"Bounded current-customer credit statement aggregate" eg:"{}"`
}
