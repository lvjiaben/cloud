package v1

import "github.com/gogf/gf/v2/frame/g"

type OperationsWorkbenchGetReq struct {
	Since  int64 `json:"since" v:"min:0" dc:"Optional previous watermark in Unix milliseconds"`
	g.Meta `path:"/admin/operations/workbench" method:"get" tags:"IDC Operations" summary:"Get operations workbench" dc:"Return bounded queue counts and original-currency financial totals in one snapshot." permission:"idc:admin:operations:workbench:view"`
}

type OperationsWorkbenchGetRes struct {
	GeneratedAt int64                       `json:"generatedAt" dc:"Snapshot time as Unix milliseconds"`
	Watermark   int64                       `json:"watermark" dc:"Source update watermark as Unix milliseconds"`
	Changed     bool                        `json:"changed" dc:"Whether data changed after the requested watermark"`
	Queues      []*OperationsQueueCount     `json:"queues" dc:"Bounded actionable queues"`
	Currencies  []*OperationsCurrencyMetric `json:"currencies" dc:"Original-currency totals; currencies are never merged without a configured rate"`
}

type OperationsReportGetReq struct {
	From         int64  `json:"from" v:"min:0" dc:"Start time Unix milliseconds; defaults to 30 days ago"`
	To           int64  `json:"to" v:"min:0" dc:"End time Unix milliseconds; defaults to now"`
	CurrencyCode string `json:"currencyCode" v:"max-length:3" dc:"Optional ISO 4217 currency code"`
	g.Meta       `path:"/admin/operations/report" method:"get" tags:"IDC Operations" summary:"Get operations report" dc:"Return original-currency report totals; no implicit cross-currency sum." permission:"idc:admin:operations:report:view"`
}

type OperationsReportGetRes struct {
	From       int64                       `json:"from"`
	To         int64                       `json:"to"`
	Currencies []*OperationsCurrencyMetric `json:"currencies"`
}

type OperationsTaskListReq struct {
	Status string `json:"status" v:"max-length:24" dc:"Optional asynchronous task status"`
	Limit  int    `json:"limit" v:"min:1|max:200" d:"50" dc:"Page size from 1 to 200"`
	g.Meta `path:"/admin/operations/tasks" method:"get" tags:"IDC Operations" summary:"List operations tasks" dc:"Return a bounded task projection without secret payloads." permission:"idc:admin:operations:tasks:view"`
}

type OperationsTaskItem struct {
	TaskNo       string `json:"taskNo"`
	TaskType     string `json:"taskType"`
	SubjectType  string `json:"subjectType"`
	SubjectNo    string `json:"subjectNo"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode"`
	ErrorSummary string `json:"errorSummary"`
	AttemptCount int    `json:"attemptCount"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type OperationsTaskListRes struct {
	Tasks []*OperationsTaskItem `json:"tasks"`
}

type OperationsTaskAlertsGetReq struct {
	g.Meta `path:"/admin/operations/alerts" method:"get" tags:"IDC Operations" summary:"List operations alerts" dc:"Return failed, unknown, or stuck asynchronous task alerts." permission:"idc:admin:operations:alerts:view"`
}

type OperationsTaskAlert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Count    int64  `json:"count"`
	OldestAt int64  `json:"oldestAt"`
}

type OperationsTaskAlertsGetRes struct {
	Alerts []*OperationsTaskAlert `json:"alerts"`
}

type OperationsQueueCount struct {
	Code  string `json:"code"`
	Count int64  `json:"count"`
}
type OperationsCurrencyMetric struct {
	CurrencyCode        string `json:"currencyCode" dc:"ISO 4217 currency code"`
	InvoiceCount        int64  `json:"invoiceCount" dc:"Invoice count in the report range"`
	TotalAmountMinor    int64  `json:"totalAmountMinor" dc:"Invoice total in the original currency minor unit"`
	PaidAmountMinor     int64  `json:"paidAmountMinor" dc:"Paid amount in the original currency minor unit"`
	RefundedAmountMinor int64  `json:"refundedAmountMinor" dc:"Refunded amount in the original currency minor unit"`
	RevenueMinor        int64  `json:"revenueMinor" dc:"Revenue amount in the original currency minor unit"`
	ReceivableMinor     int64  `json:"receivableMinor" dc:"Unpaid receivable amount in the original currency minor unit"`
	CashFlowMinor       int64  `json:"cashFlowMinor" dc:"Cash ledger movement in the original currency minor unit"`
	BalanceMinor        int64  `json:"balanceMinor" dc:"Latest balance projection in the original currency minor unit"`
	CostMinor           int64  `json:"costMinor" dc:"Frozen order cost in the original currency minor unit"`
	CostKnown           bool   `json:"costKnown" dc:"Whether all costs in this metric are frozen or corrected"`
	GrossProfitMinor    int64  `json:"grossProfitMinor" dc:"Gross profit in the original currency minor unit when cost is complete"`
	CustomerCount       int64  `json:"customerCount" dc:"Distinct customers in the report range"`
	OrderCount          int64  `json:"orderCount" dc:"Distinct orders in the report range"`
	ProductCount        int64  `json:"productCount" dc:"Distinct products in the report range"`
	SalesCount          int64  `json:"salesCount" dc:"Distinct sales attributions in the report range"`
	AffiliateCount      int64  `json:"affiliateCount" dc:"Distinct affiliate attributions in the report range"`
	SupplierSuccess     int64  `json:"supplierSuccess" dc:"Successful provider attempts in the report range"`
	SupplierFailure     int64  `json:"supplierFailure" dc:"Failed provider attempts in the report range"`
}
