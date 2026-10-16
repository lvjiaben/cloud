// This file defines independent sales performance, separate from commission rules.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementPerformanceListReq struct {
	g.Meta `path:"/admin/sales-management/performance" method:"get" tags:"IDC Sales Management" summary:"List sales performance" dc:"Group immutable payment ownership by beneficiary and currency for the selected merchant calendar month; cumulative refunds include later refunds through query time." permission:"idc:admin:sales-channel:overview:view"`
	SalesManagementPageQuery
	Month           string `json:"month" v:"required|length:7,7" dc:"Payment calendar month in merchant timezone, YYYY-MM" eg:"2026-10"`
	BeneficiaryType string `json:"beneficiaryType" v:"in:sales,affiliate" dc:"Optional owner type: sales employee or affiliate referring customer; empty returns both separately" eg:"sales"`
	CurrencyCode    string `json:"currencyCode" v:"max-length:3" dc:"Optional ISO currency filter; empty returns separate rows for every currency" eg:"CNY"`
}
type SalesManagementPerformance struct {
	BeneficiaryType      string `json:"beneficiaryType" dc:"Owner type: sales employee or affiliate referring customer" eg:"sales"`
	BeneficiaryReference string `json:"beneficiaryReference" dc:"Frozen employee ID or referring customer number" eg:"81"`
	BeneficiaryName      string `json:"beneficiaryName" dc:"Visible owner display name" eg:"Alice"`
	CurrencyCode         string `json:"currencyCode" dc:"Currency of this row; currencies are never combined" eg:"CNY"`
	PaidOrderCount       int64  `json:"paidOrderCount" dc:"Distinct settled new purchase orders, including legitimate zero-total orders" eg:"3"`
	PaidRenewalCount     int64  `json:"paidRenewalCount" dc:"Settled renewal invoice count, including legitimate zero-total renewals" eg:"1"`
	ReceivedMinor        string `json:"receivedMinor" dc:"Actual received payment amount as an integer string in currency minor units" eg:"12000"`
	RefundedMinor        string `json:"refundedMinor" dc:"Cumulative actual refunds for these payments through query time, including refunds in later months, in currency minor units" eg:"2000"`
	NetReceivedMinor     string `json:"netReceivedMinor" dc:"Received minus cumulative refunded amount in currency minor units" eg:"10000"`
}
type SalesManagementPerformanceListRes struct {
	Items    []SalesManagementPerformance `json:"items" dc:"Current page of owners and separate currency totals" eg:"[]"`
	Total    int                          `json:"total" dc:"Total matching owner and currency groups" eg:"0"`
	Timezone string                       `json:"timezone" dc:"Merchant timezone used to select the payment month" eg:"Asia/Shanghai"`
	AsOf     int64                        `json:"asOf" dc:"Refund cutoff as Unix timestamp in milliseconds" eg:"1791648000000"`
}
