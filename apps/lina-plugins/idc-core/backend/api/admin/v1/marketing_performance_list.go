package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingPerformanceListReq struct {
	g.Meta    `path:"/admin/marketing/performance" method:"get" tags:"IDC Marketing Management" summary:"List currency-safe marketing performance" dc:"Return authoritative campaign performance grouped by campaign and explicit currency for a range of at most one year." permission:"idc:admin:marketing:performance:list"`
	StartedAt int64 `json:"startedAt" v:"required|min:1" dc:"Inclusive range start as Unix milliseconds"`
	EndedAt   int64 `json:"endedAt" v:"required|min:1" dc:"Inclusive range end as Unix milliseconds"`
	Page      int   `json:"page" v:"min:1" dc:"Optional one-based page number"`
	Limit     int   `json:"limit" v:"min:1|max:100" dc:"Maximum rows per page"`
}

type MarketingPerformanceListRes struct {
	StartedAt int64                       `json:"startedAt" dc:"Accepted range start as Unix milliseconds"`
	EndedAt   int64                       `json:"endedAt" dc:"Accepted range end as Unix milliseconds"`
	Rows      []*MarketingPerformanceItem `json:"rows" dc:"Campaign and explicit-currency performance rows"`
	Total     int                         `json:"total" dc:"Exact count of authorized campaign and currency groups in the accepted range"`
}

type MarketingPerformanceItem struct {
	CampaignNo   string `json:"campaignNo"`
	NameZhCn     string `json:"nameZhCn"`
	NameEnUs     string `json:"nameEnUs"`
	CurrencyCode string `json:"currencyCode" dc:"Empty only for count-only exposure or claim facts"`
	Exposure     int64  `json:"exposure"`
	Claim        int64  `json:"claim"`
	Use          int64  `json:"use"`
	Order        int64  `json:"order"`
	ReceiptMinor string `json:"receiptMinor"`
	RefundMinor  string `json:"refundMinor"`
	CostMinor    string `json:"costMinor"`
	NetMinor     string `json:"netMinor"`
	CostComplete bool   `json:"costComplete" dc:"False when any attributed order still lacks a frozen or corrected cost"`
}
