// File admin_v1_operations_dashboard.go implements the analytics-page
// dashboard endpoints on top of the operations read model. Every handler only
// projects already-bounded aggregates; no new data-scope decisions are made
// here because the service applies the same authorizeRead gate.
package admin

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

// OperationsDashboardSummaryGet returns the revenue headline cards and the
// order-overview counters of the analytics page.
func (c *ControllerV1) OperationsDashboardSummaryGet(ctx context.Context, _ *v1.OperationsDashboardSummaryGetReq) (*v1.OperationsDashboardSummaryGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	metrics, err := c.operations.LoadDashboardSummary(ctx)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsDashboardSummaryGetRes{
		GeneratedAt: time.Now().UnixMilli(),
		Currencies:  make([]*v1.OperationsRevenueMetric, 0, len(metrics)),
	}
	for _, metric := range metrics {
		res.Currencies = append(res.Currencies, &v1.OperationsRevenueMetric{
			CurrencyCode:    metric.CurrencyCode,
			TodayMinor:      metric.TodayMinor,
			ThisMonthMinor:  metric.ThisMonthMinor,
			ThisYearMinor:   metric.ThisYearMinor,
			TotalMinor:      metric.TotalMinor,
			OrderTodayCount: metric.OrderTodayCount,
			OrderMonthCount: metric.OrderMonthCount,
			Order7DayCount:  metric.Order7DayCount,
			PendingToday:    metric.PendingToday,
			PendingMonth:    metric.PendingMonth,
			Pending7Day:     metric.Pending7Day,
		})
	}
	return res, nil
}

// OperationsCustomerStatsGet returns the customer counters of the analytics
// page.
func (c *ControllerV1) OperationsCustomerStatsGet(ctx context.Context, _ *v1.OperationsCustomerStatsGetReq) (*v1.OperationsCustomerStatsGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	stats, err := c.operations.LoadCustomerStats(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.OperationsCustomerStatsGetRes{
		GeneratedAt:     time.Now().UnixMilli(),
		TodayRegistered: stats.TodayRegistered,
		TotalCustomers:  stats.TotalCustomers,
		PendingActivate: stats.PendingActivate,
	}, nil
}

// OperationsRevenueTrendGet returns the daily paid-order revenue series used
// by the analytics-page ECharts trend chart.
func (c *ControllerV1) OperationsRevenueTrendGet(ctx context.Context, req *v1.OperationsRevenueTrendGetReq) (*v1.OperationsRevenueTrendGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	days := 30
	if req != nil {
		days = req.Days
	}
	trend, err := c.operations.LoadRevenueTrend(ctx, trendCurrencyCode(req), days)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsRevenueTrendGetRes{
		CurrencyCode: trendCurrencyCode(req),
		Days:         make([]*v1.OperationsRevenueTrendDay, 0, len(trend)),
	}
	for _, day := range trend {
		res.Days = append(res.Days, &v1.OperationsRevenueTrendDay{
			Date:          day.Date,
			CurrentMinor:  day.CurrentMinor,
			CurrentCount:  day.CurrentCount,
			PreviousMinor: day.PreviousMinor,
			PreviousCount: day.PreviousCount,
		})
	}
	return res, nil
}

func trendCurrencyCode(req *v1.OperationsRevenueTrendGetReq) string {
	if req == nil {
		return ""
	}
	return req.CurrencyCode
}

// OperationsExpiringServicesGet returns active service counts per product
// type whose entitlement expires within the next 7 days.
func (c *ControllerV1) OperationsExpiringServicesGet(ctx context.Context, _ *v1.OperationsExpiringServicesGetReq) (*v1.OperationsExpiringServicesGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	services, err := c.operations.LoadExpiringServices(ctx)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsExpiringServicesGetRes{Services: make([]*v1.OperationsExpiringService, 0, len(services))}
	for _, service := range services {
		res.Services = append(res.Services, &v1.OperationsExpiringService{
			ProductTypeCode: service.ProductTypeCode,
			Count:           service.Count,
		})
	}
	return res, nil
}

// OperationsProductSalesRankGet returns the product sales ranking of the last
// 30 days.
func (c *ControllerV1) OperationsProductSalesRankGet(ctx context.Context, _ *v1.OperationsProductSalesRankGetReq) (*v1.OperationsProductSalesRankGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	products, err := c.operations.LoadProductSalesRank(ctx, 10)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsProductSalesRankGetRes{Products: make([]*v1.OperationsProductSalesRank, 0, len(products))}
	for _, product := range products {
		res.Products = append(res.Products, &v1.OperationsProductSalesRank{
			ProductNo:   product.ProductNo,
			ProductName: product.ProductName,
			SalesCount:  product.SalesCount,
		})
	}
	return res, nil
}
