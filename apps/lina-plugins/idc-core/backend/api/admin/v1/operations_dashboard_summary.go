package v1

import "github.com/gogf/gf/v2/frame/g"

// OperationsDashboardSummaryGetReq asks for the paid-order revenue headline
// cards on the operations analytics page.
type OperationsDashboardSummaryGetReq struct {
	g.Meta `path:"/admin/operations/dashboard/summary" method:"get" tags:"IDC Operations" summary:"Get operations dashboard summary" dc:"Return today, this month, this year, and all-time paid-order revenue totals plus order-overview counts for the operations analytics page. Amounts are grouped by original currency and never merged." permission:"idc:admin:operations:workbench:view"`
}

// OperationsRevenueMetric is one original-currency revenue headline.
type OperationsRevenueMetric struct {
	CurrencyCode    string `json:"currencyCode" dc:"ISO 4217 currency code of the revenue totals" eg:"CNY"`
	TodayMinor      int64  `json:"todayMinor" dc:"Paid-order revenue today in the original currency minor unit" eg:"6194580"`
	ThisMonthMinor  int64  `json:"thisMonthMinor" dc:"Paid-order revenue this month in the original currency minor unit" eg:"87166930"`
	ThisYearMinor   int64  `json:"thisYearMinor" dc:"Paid-order revenue this year in the original currency minor unit" eg:"447505585"`
	TotalMinor      int64  `json:"totalMinor" dc:"All-time paid-order revenue in the original currency minor unit" eg:"1200000000"`
	OrderTodayCount int64  `json:"orderTodayCount" dc:"Orders placed today, all statuses included" eg:"3"`
	OrderMonthCount int64  `json:"orderMonthCount" dc:"Orders placed this month, all statuses included" eg:"111"`
	Order7DayCount  int64  `json:"order7DayCount" dc:"Orders placed in the last 7 days, all statuses included" eg:"31"`
	PendingToday    int64  `json:"pendingToday" dc:"Orders placed today that are still pending payment" eg:"1"`
	PendingMonth    int64  `json:"pendingMonth" dc:"Orders placed this month that are still pending payment" eg:"19"`
	Pending7Day     int64  `json:"pending7Day" dc:"Orders placed in the last 7 days that are still pending payment" eg:"0"`
}

// OperationsDashboardSummaryGetRes carries the revenue cards and the
// order-overview counters of the analytics page.
type OperationsDashboardSummaryGetRes struct {
	GeneratedAt int64                      `json:"generatedAt" dc:"Snapshot time as Unix timestamp in milliseconds" eg:"1770000000000"`
	Currencies  []*OperationsRevenueMetric `json:"currencies" dc:"Revenue totals grouped by original currency"`
}

// OperationsCustomerStatsGetReq asks for the customer counters of the
// analytics page.
type OperationsCustomerStatsGetReq struct {
	g.Meta `path:"/admin/operations/dashboard/customers" method:"get" tags:"IDC Operations" summary:"Get operations customer statistics" dc:"Return today's registrations, total customers, and pending-activation customer counts for the operations analytics page." permission:"idc:admin:operations:workbench:view"`
}

// OperationsCustomerStatsGetRes carries the customer counters.
type OperationsCustomerStatsGetRes struct {
	GeneratedAt     int64 `json:"generatedAt" dc:"Snapshot time as Unix timestamp in milliseconds" eg:"1770000000000"`
	TodayRegistered int64 `json:"todayRegistered" dc:"Customers created today" eg:"2"`
	TotalCustomers  int64 `json:"totalCustomers" dc:"All customers regardless of status" eg:"104"`
	PendingActivate int64 `json:"pendingActivate" dc:"Customers still waiting for activation" eg:"9"`
}

// OperationsRevenueTrendGetReq asks for the daily paid-order revenue series
// of a moving window used by the ECharts trend chart.
type OperationsRevenueTrendGetReq struct {
	g.Meta `path:"/admin/operations/dashboard/revenue-trend" method:"get" tags:"IDC Operations" summary:"Get daily operations revenue trend" dc:"Return the daily paid-order revenue series of a moving window plus the comparison window, grouped by original currency." permission:"idc:admin:operations:workbench:view"`
	CurrencyCode string `json:"currencyCode" in:"query" v:"required|max-length:3" dc:"ISO 4217 currency code of the trend series" eg:"CNY"`
	Days         int    `json:"days" in:"query" d:"30" v:"between:7,31" dc:"Window length in days including today; 30 by default" eg:"30"`
}

// OperationsRevenueTrendDay is one day of the trend series.
type OperationsRevenueTrendDay struct {
	Date          string `json:"date" dc:"Calendar day in YYYY-MM-DD in the merchant session timezone" eg:"2026-09-24"`
	CurrentMinor  int64  `json:"currentMinor" dc:"Paid-order revenue of the current window day in the original currency minor unit" eg:"15800"`
	CurrentCount  int64  `json:"currentCount" dc:"Paid orders of the current window day" eg:"2"`
	PreviousMinor int64  `json:"previousMinor" dc:"Paid-order revenue of the same calendar day one year earlier, or of the preceding window when the previous window does not exist, in the original currency minor unit" eg:"9000"`
	PreviousCount int64  `json:"previousCount" dc:"Paid orders of the same comparison day as previousMinor" eg:"1"`
}

// OperationsRevenueTrendGetRes carries the daily revenue series.
type OperationsRevenueTrendGetRes struct {
	CurrencyCode string                       `json:"currencyCode" dc:"ISO 4217 currency code of the series; empty when no paid order exists yet" eg:"CNY"`
	Days         []*OperationsRevenueTrendDay `json:"days" dc:"Daily series ordered from the earliest window day to today; one entry per day"`
}

// OperationsExpiringService is one expiring-service bucket of the analytics
// page.
type OperationsExpiringService struct {
	ProductTypeCode string `json:"productTypeCode" dc:"Stable product type code of the expiring services" eg:"cloud_server"`
	Count           int64  `json:"count" dc:"Active services expiring within the next 7 days" eg:"6"`
}

// OperationsExpiringServicesGetReq asks for the expiring-service projection.
type OperationsExpiringServicesGetReq struct {
	g.Meta `path:"/admin/operations/dashboard/expiring-services" method:"get" tags:"IDC Operations" summary:"List expiring services" dc:"Return active service counts per product type whose entitlement expires within the next 7 days, across all plugin-owned services." permission:"idc:admin:operations:workbench:view"`
}

// OperationsExpiringServicesGetRes carries the expiring-service buckets.
type OperationsExpiringServicesGetRes struct {
	Services []*OperationsExpiringService `json:"services" dc:"Expiring-service counts per product type"`
}

// OperationsProductSalesRank is one product of the sales ranking.
type OperationsProductSalesRank struct {
	ProductNo   string `json:"productNo" dc:"Stable product business number" eg:"PRD-2026-000001"`
	ProductName string `json:"productName" dc:"Product display name in the merchant's current locale" eg:"Cloud Server Standard"`
	SalesCount  int64  `json:"salesCount" dc:"Paid order items of the product in the last 30 days" eg:"25"`
}

// OperationsProductSalesRankGetReq asks for the product sales ranking.
type OperationsProductSalesRankGetReq struct {
	g.Meta `path:"/admin/operations/dashboard/product-sales" method:"get" tags:"IDC Operations" summary:"List product sales ranking" dc:"Return the top products ranked by paid order items over the last 30 days." permission:"idc:admin:operations:workbench:view"`
}

// OperationsProductSalesRankGetRes carries the ranked products.
type OperationsProductSalesRankGetRes struct {
	Products []*OperationsProductSalesRank `json:"products" dc:"Products ordered by descending sales count"`
}
