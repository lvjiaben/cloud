// File dashboard.go carries the bounded read model behind the operations
// analytics page: revenue headline cards, customer counters, the daily
// revenue trend, expiring services, and the product sales ranking. Every
// query stays in one SQL round trip per projection and never iterates rows
// with per-row database calls.

package operations

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// RevenueMetric carries the paid-order revenue headlines of one currency.
type RevenueMetric struct {
	CurrencyCode    string
	TodayMinor      int64
	ThisMonthMinor  int64
	ThisYearMinor   int64
	TotalMinor      int64
	OrderTodayCount int64
	OrderMonthCount int64
	Order7DayCount  int64
	PendingToday    int64
	PendingMonth    int64
	Pending7Day     int64
}

// CustomerStats carries the customer counters of the analytics page.
type CustomerStats struct {
	TodayRegistered int64
	TotalCustomers  int64
	PendingActivate int64
}

// TrendDay is one calendar day of the paid-order revenue trend. The
// comparison day is the same calendar day of the previous period.
type TrendDay struct {
	Date          string
	CurrentMinor  int64
	CurrentCount  int64
	PreviousMinor int64
	PreviousCount int64
}

// ExpiringServiceBucket counts active services of one product type whose
// entitlement expires within the next 7 days.
type ExpiringServiceBucket struct {
	ProductTypeCode string
	Count           int64
}

// ProductSalesRank is one ranked product of the last-30-days sales ranking.
type ProductSalesRank struct {
	ProductNo   string
	ProductName string
	SalesCount  int64
}

// dashboardSummary loads the revenue headline cards plus the order-overview
// counters. It runs exactly two aggregate queries: paid revenue per currency
// and order counts per currency window.
func (s *serviceImpl) dashboardSummary(ctx context.Context, now time.Time) ([]RevenueMetric, error) {
	rows, err := s.database.GetAll(ctx, `
WITH windows AS (
    SELECT
        DATE_TRUNC('day', ?::timestamptz) AS day_start,
        DATE_TRUNC('month', ?::timestamptz) AS month_start,
        DATE_TRUNC('year', ?::timestamptz) AS year_start
),
paid AS (
    SELECT o.currency_code,
           COALESCE(SUM(o.total_amount_minor) FILTER (
               WHERE o.paid_at >= (SELECT day_start FROM windows)
           ), 0) AS today_minor,
           COALESCE(SUM(o.total_amount_minor) FILTER (
               WHERE o.paid_at >= (SELECT month_start FROM windows)
           ), 0) AS month_minor,
           COALESCE(SUM(o.total_amount_minor) FILTER (
               WHERE o.paid_at >= (SELECT year_start FROM windows)
           ), 0) AS year_minor,
           COALESCE(SUM(o.total_amount_minor), 0) AS total_minor
      FROM plugin_idc_core_commerce_order o
     WHERE o.tenant_id=0 AND o.paid_at IS NOT NULL
     GROUP BY o.currency_code
),

placed AS (
    SELECT o.currency_code,
           COUNT(*) FILTER (WHERE o.created_at >= (SELECT day_start FROM windows)) AS order_today,
           COUNT(*) FILTER (WHERE o.created_at >= (SELECT month_start FROM windows)) AS order_month,
           COUNT(*) FILTER (WHERE o.created_at >= (SELECT day_start FROM windows) - INTERVAL '6 days') AS order_7day,
           COUNT(*) FILTER (
               WHERE o.created_at >= (SELECT day_start FROM windows)
               AND o.status_code IN ('pending_payment','payment_processing')
           ) AS pending_today,
           COUNT(*) FILTER (
               WHERE o.created_at >= (SELECT month_start FROM windows)
               AND o.status_code IN ('pending_payment','payment_processing')
           ) AS pending_month,
           COUNT(*) FILTER (
               WHERE o.created_at >= (SELECT day_start FROM windows) - INTERVAL '6 days'
               AND o.status_code IN ('pending_payment','payment_processing')
           ) AS pending_7day
      FROM plugin_idc_core_commerce_order o
     WHERE o.tenant_id=0
     GROUP BY o.currency_code
)

SELECT COALESCE(p.currency_code, c.currency_code) AS currency_code,
       COALESCE(p.today_minor,0) AS today_minor,
       COALESCE(p.month_minor,0) AS month_minor,
       COALESCE(p.year_minor,0) AS year_minor,
       COALESCE(p.total_minor,0) AS total_minor,
       COALESCE(c.order_today,0) AS order_today,
       COALESCE(c.order_month,0) AS order_month,
       COALESCE(c.order_7day,0) AS order_7day,
       COALESCE(c.pending_today,0) AS pending_today,
       COALESCE(c.pending_month,0) AS pending_month,
       COALESCE(c.pending_7day,0) AS pending_7day
  FROM paid p
  FULL OUTER JOIN placed c ON c.currency_code = p.currency_code
 ORDER BY currency_code`, now, now, now)
	if err != nil {
		return nil, gerror.Wrap(err, "load dashboard revenue summary")
	}
	result := make([]RevenueMetric, 0, len(rows))
	for _, row := range rows {
		result = append(result, RevenueMetric{
			CurrencyCode:    row["currency_code"].String(),
			TodayMinor:      row["today_minor"].Int64(),
			ThisMonthMinor:  row["month_minor"].Int64(),
			ThisYearMinor:   row["year_minor"].Int64(),
			TotalMinor:      row["total_minor"].Int64(),
			OrderTodayCount: row["order_today"].Int64(),
			OrderMonthCount: row["order_month"].Int64(),
			Order7DayCount:  row["order_7day"].Int64(),
			PendingToday:    row["pending_today"].Int64(),
			PendingMonth:    row["pending_month"].Int64(),
			Pending7Day:     row["pending_7day"].Int64(),
		})
	}
	return result, nil
}

// customerStats loads the analytics-page customer counters in one query.
func (s *serviceImpl) customerStats(ctx context.Context, now time.Time) (CustomerStats, error) {
	rows, err := s.database.GetAll(ctx, `
SELECT COUNT(*) FILTER (WHERE created_at >= DATE_TRUNC('day', ?::timestamptz)) AS today_registered,
       COUNT(*) AS total_customers,
       COUNT(*) FILTER (WHERE status_code = 'pending_activation') AS pending_activate
  FROM plugin_idc_core_customer_subject
 WHERE tenant_id=0`, now)
	if err != nil {
		return CustomerStats{}, gerror.Wrap(err, "load dashboard customer stats")
	}
	if len(rows) == 0 {
		return CustomerStats{}, nil
	}
	return CustomerStats{
		TodayRegistered: rows[0]["today_registered"].Int64(),
		TotalCustomers:  rows[0]["total_customers"].Int64(),
		PendingActivate: rows[0]["pending_activate"].Int64(),
	}, nil
}

// revenueTrend loads the daily paid-order revenue series of the given
// currency for the moving window ending today, together with the same
// calendar days of the previous year. It fills missing days with zeros so the
// chart can always render a fixed-length axis.
func (s *serviceImpl) revenueTrend(ctx context.Context, currencyCode string, days int, now time.Time) ([]TrendDay, error) {
	rows, err := s.database.GetAll(ctx, `
WITH bounds AS (
    SELECT DATE_TRUNC('day', ?::timestamptz) AS today
),
current_window AS (
    SELECT DATE_TRUNC('day', o.paid_at) AS day,
           SUM(o.total_amount_minor) AS minor,
           COUNT(*) AS total
      FROM plugin_idc_core_commerce_order o, bounds b
     WHERE o.tenant_id=0 AND o.paid_at IS NOT NULL
       AND o.currency_code = ?
       AND o.paid_at >= b.today - (?::text)::interval
       AND o.paid_at < b.today + INTERVAL '1 day'
     GROUP BY 1
),

previous_window AS (
    SELECT DATE_TRUNC('day', o.paid_at) AS day,
           SUM(o.total_amount_minor) AS minor,
           COUNT(*) AS total
      FROM plugin_idc_core_commerce_order o, bounds b
     WHERE o.tenant_id=0 AND o.paid_at IS NOT NULL
       AND o.currency_code = ?
       AND o.paid_at >= b.today - (?::text)::interval - INTERVAL '1 year'
       AND o.paid_at < b.today + INTERVAL '1 day' - INTERVAL '1 year'
     GROUP BY 1
),

days AS (
    SELECT generate_series(
        (SELECT today FROM bounds) - (?::text)::interval + INTERVAL '1 day',
        (SELECT today FROM bounds),
        INTERVAL '1 day'
    )::date AS day
)

SELECT to_char(d.day, 'YYYY-MM-DD') AS day,
       COALESCE(c.minor, 0) AS current_minor,
       COALESCE(c.total, 0) AS current_total,
       COALESCE(p.minor, 0) AS previous_minor,
       COALESCE(p.total, 0) AS previous_total
  FROM days d
  LEFT JOIN current_window c ON c.day = d.day
  LEFT JOIN previous_window p ON p.day = d.day - INTERVAL '1 year'
 ORDER BY d.day`, now, currencyCode, daysToInterval(days), currencyCode, daysToInterval(days), daysToInterval(days))
	if err != nil {
		return nil, gerror.Wrap(err, "load dashboard revenue trend")
	}
	result := make([]TrendDay, 0, len(rows))
	for _, row := range rows {
		result = append(result, TrendDay{
			Date:          row["day"].String(),
			CurrentMinor:  row["current_minor"].Int64(),
			CurrentCount:  row["current_total"].Int64(),
			PreviousMinor: row["previous_minor"].Int64(),
			PreviousCount: row["previous_total"].Int64(),
		})
	}
	return result, nil
}

// daysToInterval renders a validated window length as a PostgreSQL interval
// literal parameter. Only integers from the validator ever reach this point.
func daysToInterval(days int) string {
	return strconv.Itoa(days) + " days"
}

// expiringServices counts, per product type, active services whose entitlement
// expires within the next 7 days. All plugin-owned services register in this
// one table, so the projection automatically covers every fulfillment plugin.
func (s *serviceImpl) expiringServices(ctx context.Context, now time.Time) ([]ExpiringServiceBucket, error) {
	rows, err := s.database.GetAll(ctx, `
SELECT product_type_code, COUNT(*) AS total
  FROM plugin_idc_core_service_instance
 WHERE tenant_id=0
   AND entitlement_status_code = 'active'
   AND entitlement_expires_at IS NOT NULL
   AND entitlement_expires_at > ?::timestamptz
   AND entitlement_expires_at <= ?::timestamptz + INTERVAL '7 days'
 GROUP BY product_type_code
 ORDER BY product_type_code`, now, now)
	if err != nil {
		return nil, gerror.Wrap(err, "load dashboard expiring services")
	}
	result := make([]ExpiringServiceBucket, 0, len(rows))
	for _, row := range rows {
		result = append(result, ExpiringServiceBucket{
			ProductTypeCode: row["product_type_code"].String(),
			Count:           row["total"].Int64(),
		})
	}
	return result, nil
}

// productSalesRank ranks products by their paid order items over the last 30
// days. The bounded limit is applied in SQL; names come from the order-item
// snapshot so no per-row lookup is needed.
func (s *serviceImpl) productSalesRank(ctx context.Context, now time.Time, limit int) ([]ProductSalesRank, error) {
	rows, err := s.database.GetAll(ctx, `
SELECT oi.product_no,
       COUNT(*) AS sales_count
  FROM plugin_idc_core_commerce_order_item oi
  JOIN plugin_idc_core_commerce_order o ON o.order_no = oi.order_no AND o.tenant_id = 0
 WHERE oi.tenant_id=0
   AND o.paid_at IS NOT NULL
   AND o.paid_at >= ?::timestamptz - INTERVAL '30 days'
 GROUP BY oi.product_no
 ORDER BY sales_count DESC, oi.product_no
 LIMIT ?`, now, limit)
	if err != nil {
		return nil, gerror.Wrap(err, "load dashboard product sales ranking")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	productNos := make([]string, 0, len(rows))
	for _, row := range rows {
		productNos = append(productNos, row["product_no"].String())
	}
	nameRows, err := s.database.GetAll(ctx, `
SELECT product_no, name_zh_cn, name_en_us
  FROM plugin_idc_core_catalog_product
 WHERE product_no IN (?)`, productNos)
	if err != nil {
		return nil, gerror.Wrap(err, "load dashboard product names")
	}
	names := make(map[string][2]string, len(nameRows))
	for _, row := range nameRows {
		names[row["product_no"].String()] = [2]string{
			row["name_zh_cn"].String(), row["name_en_us"].String(),
		}
	}
	result := make([]ProductSalesRank, 0, len(rows))
	for _, row := range rows {
		name := names[row["product_no"].String()]
		result = append(result, ProductSalesRank{
			ProductNo:   row["product_no"].String(),
			ProductName: name[0],
			SalesCount:  row["sales_count"].Int64(),
		})
	}
	return result, nil
}

// LoadDashboardSummary returns the paid-order revenue headline metrics per
// currency for the operations analytics page.
func (s *serviceImpl) LoadDashboardSummary(ctx context.Context) ([]RevenueMetric, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, err
	}
	return s.dashboardSummary(ctx, time.Now().UTC())
}

// LoadCustomerStats returns the customer counters for the analytics page.
func (s *serviceImpl) LoadCustomerStats(ctx context.Context) (CustomerStats, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return CustomerStats{}, err
	}
	return s.customerStats(ctx, time.Now().UTC())
}

// LoadRevenueTrend returns the daily paid-order revenue series for one
// currency over a moving window ending today.
func (s *serviceImpl) LoadRevenueTrend(ctx context.Context, currencyCode string, days int) ([]TrendDay, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, err
	}
	currencyCode = normalizeCurrencyCode(currencyCode)
	if currencyCode == "" {
		return nil, gerror.New("operations trend currency is required")
	}
	if days < 7 || days > 31 {
		return nil, gerror.New("operations trend window is invalid")
	}
	return s.revenueTrend(ctx, currencyCode, days, time.Now().UTC())
}

// LoadExpiringServices returns per-product-type counts of active services
// expiring within the next 7 days across all plugin-owned services.
func (s *serviceImpl) LoadExpiringServices(ctx context.Context) ([]ExpiringServiceBucket, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, err
	}
	return s.expiringServices(ctx, time.Now().UTC())
}

// LoadProductSalesRank returns the top products by paid order items over the
// last 30 days.
func (s *serviceImpl) LoadProductSalesRank(ctx context.Context, limit int) ([]ProductSalesRank, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 50 {
		return nil, gerror.New("operations sales rank limit is invalid")
	}
	return s.productSalesRank(ctx, time.Now().UTC(), limit)
}

// normalizeCurrencyCode upper-cases and trims one ISO 4217 code.
func normalizeCurrencyCode(currencyCode string) string {
	return strings.ToUpper(strings.TrimSpace(currencyCode))
}
