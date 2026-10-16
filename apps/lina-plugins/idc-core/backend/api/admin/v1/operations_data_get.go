// This file defines the operating-data overview contract and documented projections.

package v1

import "github.com/gogf/gf/v2/frame/g"

// OperationsDataPeriod documents the operating-data dataperiod projection.
type OperationsDataPeriod struct {
	Key         string `json:"key" dc:"Period: today, yesterday, month or all" eg:"today"`
	StartAt     int64  `json:"startAt" dc:"Inclusive start, Unix timestamp in milliseconds; zero means all history" eg:"0"`
	EndAt       int64  `json:"endAt" dc:"Exclusive end, Unix timestamp in milliseconds" eg:"1791561600000"`
	IncomeMinor string `json:"incomeMinor" dc:"Actual cash receipts in minor units" eg:"10000"`
	PaidOrders  int64  `json:"paidOrders" dc:"Number of paid orders" eg:"5"`
}

// OperationsDataTrend documents the operating-data datatrend projection.
type OperationsDataTrend struct {
	Date            string `json:"date" dc:"Merchant calendar date, date-only YYYY-MM-DD" eg:"2026-10-01"`
	PreviousDate    string `json:"previousDate" dc:"Previous month comparison date, date-only; empty when absent" eg:"2026-09-01"`
	StartAt         int64  `json:"startAt" dc:"Inclusive day start, Unix timestamp in milliseconds" eg:"1790784000000"`
	EndAt           int64  `json:"endAt" dc:"Exclusive day end, Unix timestamp in milliseconds" eg:"1790870400000"`
	PreviousStartAt int64  `json:"previousStartAt" dc:"Comparison start, Unix timestamp in milliseconds; zero when absent" eg:"1788192000000"`
	PreviousEndAt   int64  `json:"previousEndAt" dc:"Comparison end, Unix timestamp in milliseconds; zero when absent" eg:"1788278400000"`
	CurrentMinor    string `json:"currentMinor" dc:"Actual daily receipts in minor units" eg:"10000"`
	CurrentCount    int64  `json:"currentCount" dc:"Current month paid order count" eg:"5"`
	PreviousCount   *int64 `json:"previousCount" dc:"Previous month paid order count; null for absent calendar day" eg:"3"`
}

// OperationsDataProduct documents the operating-data dataproduct projection.
type OperationsDataProduct struct {
	ProductNo string `json:"productNo" dc:"Product business number" eg:"PRD_1"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese product snapshot name" eg:"Cloud server"`
	NameEnUS  string `json:"nameEnUS" dc:"English product snapshot name" eg:"Cloud server"`
	Quantity  int64  `json:"quantity" dc:"Paid quantity during the calendar month" eg:"5"`
}

// OperationsDataCustomers documents the operating-data datacustomers projection.
type OperationsDataCustomers struct {
	Today  int64 `json:"today" dc:"Customers registered today" eg:"1"`
	Total  int64 `json:"total" dc:"Total customers" eg:"10"`
	Online int64 `json:"online" dc:"Distinct customers with effective online sessions" eg:"2"`
}

// OperationsDataTickets documents the operating-data datatickets projection.
type OperationsDataTickets struct {
	Pending   int64 `json:"pending" dc:"Tickets neither resolved nor closed" eg:"2"`
	Processed int64 `json:"processed" dc:"Resolved or closed tickets" eg:"4"`
}

// OperationsDataRecord documents the operating-data datarecord projection.
type OperationsDataRecord struct {
	ID           string `json:"id" dc:"Business number for opening the original detail" eg:"ORD_1"`
	Label        string `json:"label" dc:"Display name or source reference" eg:"ORD_1"`
	CustomerNo   string `json:"customerNo" dc:"Related customer business number, when available" eg:"CUS_1"`
	CustomerName string `json:"customerName" dc:"Related customer display name, when available" eg:"Example"`
	CurrencyCode string `json:"currencyCode" dc:"Original currency; empty for nonmonetary records" eg:"CNY"`
	AmountMinor  string `json:"amountMinor" dc:"Amount in original currency minor units" eg:"10000"`
	Quantity     int64  `json:"quantity" dc:"Purchased quantity for product records" eg:"1"`
	StatusCode   string `json:"statusCode" dc:"Domain status for the selected record kind" eg:"paid"`
	OccurredAt   int64  `json:"occurredAt" dc:"Metric event time, Unix timestamp in milliseconds" eg:"1791561600000"`
}

// OperationsData documents the operating-data data projection.
type OperationsData struct {
	Currencies       []string                `json:"currencies" dc:"Available original currencies" eg:"[&quot;CNY&quot;]"`
	CurrencyCode     string                  `json:"currencyCode" dc:"Selected original currency" eg:"CNY"`
	CurrencyExponent int                     `json:"currencyExponent" dc:"Number of minor-unit decimal places" eg:"2"`
	Timezone         string                  `json:"timezone" dc:"Merchant IANA timezone" eg:"Asia/Shanghai"`
	AsOf             int64                   `json:"asOf" dc:"Read boundary, Unix timestamp in milliseconds" eg:"1791561600000"`
	Periods          []OperationsDataPeriod  `json:"periods" dc:"Today, yesterday, month and all-history metrics" eg:"[]"`
	Trend            []OperationsDataTrend   `json:"trend" dc:"Calendar-month daily comparison" eg:"[]"`
	Products         []OperationsDataProduct `json:"products" dc:"Top ten products by purchased quantity" eg:"[]"`
	Customers        OperationsDataCustomers `json:"customers" dc:"Customer counters" eg:"{}"`
	ExpiringServers  int64                   `json:"expiringServers" dc:"Active servers expiring in the next seven days" eg:"2"`
	Tickets          OperationsDataTickets   `json:"tickets" dc:"Current ticket counters" eg:"{}"`
}

// OperationsDataRecords documents the operating-data datarecords projection.
type OperationsDataRecords struct {
	Items []OperationsDataRecord `json:"items" dc:"Bounded matching records" eg:"[]"`
	Total int64                  `json:"total" dc:"Exact number of matching records" eg:"0"`
}

// OperationsDataGetReq reads all homepage metrics in one bounded request.
type OperationsDataGetReq struct {
	g.Meta       `path:"/admin/operations/data" method:"get" tags:"IDC Operations" summary:"Get operating data" dc:"Read actual receipts, paid orders and customer and service metrics in the merchant timezone without mixing currencies." permission:"idc:admin:operations:workbench:view"`
	CurrencyCode string `json:"currencyCode" dc:"Original currency; merchant default when omitted" eg:"CNY"`
}

// OperationsDataGetRes is the complete overview.
type OperationsDataGetRes struct{ OperationsData }
