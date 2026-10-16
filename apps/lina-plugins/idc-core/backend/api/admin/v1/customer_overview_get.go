package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerOverviewGetReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}" method:"get" tags:"IDC Customer Management" summary:"Get customer overview" dc:"Return the bounded, display-safe local customer-domain foundation for the management customer 360 view." permission:"idc:admin:customer:query"`
	CustomerNo string `json:"customerNo" v:"required" dc:"Stable external customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
}

type CustomerOverviewGetRes struct {
	Statistics *CustomerOperationsStatistics `json:"statistics" dc:"Customer financial, ticket and product statistics" eg:"{}"`
	Customer   *CustomerSummaryItem          `json:"customer" dc:"Display-safe customer summary" eg:"{}"`
	Groups     *CustomerSegmentPreview       `json:"groups" dc:"Bounded customer group preview" eg:"{}"`
	Tags       *CustomerSegmentPreview       `json:"tags" dc:"Bounded customer tag preview" eg:"{}"`
	Counts     *CustomerOverviewCounts       `json:"counts" dc:"Local customer-domain aggregate counts" eg:"{}"`
	Business   *CustomerBusinessSummary      `json:"business" dc:"Cross-domain customer business counts" eg:"{}"`
	Balances   []*CustomerBalanceSummary     `json:"balances" dc:"Display-safe customer balances" eg:"[]"`
}
