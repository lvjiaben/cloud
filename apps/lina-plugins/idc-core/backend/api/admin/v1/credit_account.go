// This file defines credit account list and detail management contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ManagementCreditAccountListReq lists bounded credit accounts after exact authorization.
type ManagementCreditAccountListReq struct {
	Page         int `json:"page" v:"min:0" dc:"Optional one-based page number; omit or zero for cursor paging; cannot combine with cursor" eg:"1"`
	g.Meta       `path:"/admin/billing/credit/accounts" method:"get" tags:"IDC Credit Management" summary:"List credit accounts" dc:"Return one bounded page and exact filtered total after exact account-list permission." permission:"idc:admin:billing:credit-account:list"`
	CustomerNo   string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number" eg:"CUS_01J2ABCDEF"`
	StatusCode   string `json:"statusCode" dc:"Optional exact account status: pending_activation, active, frozen, expired, closed, or bad_debt" eg:"frozen"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum accounts returned in this page" eg:"20"`
}

// ManagementCreditAccountListRes returns one bounded account page.
type ManagementCreditAccountListRes struct {
	Total      int                        `json:"total" dc:"Exact authorized record count matching the current filters before pagination" eg:"123"`
	Accounts   []*ManagementCreditAccount `json:"accounts" dc:"Bounded authorized credit accounts" eg:"[]"`
	NextCursor string                     `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

// ManagementCreditAccountDetailReq requests one account after exact detail authorization.
type ManagementCreditAccountDetailReq struct {
	g.Meta          `path:"/admin/billing/credit/accounts/{creditAccountNo}" method:"get" tags:"IDC Credit Management" summary:"Get credit account detail" dc:"Return one account, current terms, and latest delinquency after exact detail permission." permission:"idc:admin:billing:credit-account:query"`
	CreditAccountNo string `json:"creditAccountNo" in:"path" v:"required|max-length:40" dc:"Stable credit account number" eg:"CRA_01J2ABCDEF"`
}

// ManagementCreditAccountDetailRes returns one authorized credit account aggregate.
type ManagementCreditAccountDetailRes struct {
	Account *ManagementCreditAccount `json:"account" dc:"Authorized credit account aggregate" eg:"{}"`
}
