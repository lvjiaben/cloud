// This file computes operating metrics from shared row sources, using explicit
// merchant calendar boundaries and a fixed number of aggregate database calls.

package operations

import (
	"context"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/currency"
	"lina-core/pkg/bizerr"
)

// calendarPeriods constructs half-open windows in the merchant timezone.
func calendarPeriods(now time.Time) []DataPeriod {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return []DataPeriod{
		{Key: "today", StartAt: day.UnixMilli(), EndAt: now.UnixMilli(), IncomeMinor: "0"},
		{Key: "yesterday", StartAt: day.AddDate(0, 0, -1).UnixMilli(), EndAt: day.UnixMilli(), IncomeMinor: "0"},
		{Key: "month", StartAt: month.UnixMilli(), EndAt: now.UnixMilli(), IncomeMinor: "0"},
		{Key: "all", EndAt: now.UnixMilli(), IncomeMinor: "0"},
	}
}

// calendarTrend retains every current-month date and never normalizes an absent
// previous-month day into the following month (including leap-year February).
func calendarTrend(now time.Time) []DataTrend {
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	previous := month.AddDate(0, -1, 0)
	next := month.AddDate(0, 1, 0)
	result := make([]DataTrend, 0, 31)
	for day := month; day.Before(next); day = day.AddDate(0, 0, 1) {
		item := DataTrend{Date: day.Format(time.DateOnly), StartAt: day.UnixMilli(), EndAt: day.AddDate(0, 0, 1).UnixMilli(), CurrentMinor: "0"}
		comparison := previous.AddDate(0, 0, day.Day()-1)
		if comparison.Before(month) {
			zero := int64(0)
			item.PreviousDate = comparison.Format(time.DateOnly)
			item.PreviousStartAt = comparison.UnixMilli()
			item.PreviousEndAt = comparison.AddDate(0, 0, 1).UnixMilli()
			item.PreviousCount = &zero
		}
		result = append(result, item)
	}
	return result
}

// LoadData reads authoritative facts without mixing currencies or materializing
// unbounded business rows. Errors do not turn unavailable metrics into zeros.
func (s *serviceImpl) LoadData(ctx context.Context, code string) (result Data, err error) {
	defer func() { err = dataError(err) }()
	if err = s.authorizeData(ctx); err != nil {
		return result, bizerr.NewCode(CodeDataDenied)
	}
	merchant, err := s.database.GetOne(ctx, "SELECT timezone,default_currency FROM plugin_idc_core_merchant_profile WHERE tenant_id=0 AND enabled=true LIMIT 1")
	if err != nil {
		return result, err
	}
	if len(merchant) == 0 {
		return result, bizerr.NewCode(CodeDataUnavailable)
	}
	location, err := time.LoadLocation(merchant["timezone"].String())
	if err != nil {
		return result, err
	}
	code = normalizeCurrencyCode(code)
	if code == "" {
		code = merchant["default_currency"].String()
	}
	unit, err := currency.ParseISO(code)
	if err != nil || unit == currency.XXX || !validCurrencyCode(code) {
		return result, bizerr.NewCode(CodeDataInvalid)
	}
	now := time.Now().In(location).Truncate(time.Millisecond)
	result = Data{CurrencyCode: code, Timezone: location.String(), AsOf: now.UnixMilli(), Periods: calendarPeriods(now), Trend: calendarTrend(now), Products: []DataProduct{}, Currencies: []string{}}
	result.CurrencyExponent, _ = currency.Standard.Rounding(unit)
	currencies, err := s.database.GetAll(ctx, `SELECT currency_code FROM plugin_idc_core_billing_ledger_transaction WHERE tenant_id=0
 UNION SELECT currency_code FROM plugin_idc_core_commerce_order WHERE tenant_id=0
 UNION SELECT default_currency FROM plugin_idc_core_merchant_profile WHERE tenant_id=0 ORDER BY currency_code LIMIT 200`)
	if err != nil {
		return result, err
	}
	for _, row := range currencies {
		result.Currencies = append(result.Currencies, row["currency_code"].String())
	}
	if err = s.loadPeriodMetrics(ctx, &result); err != nil {
		return result, err
	}
	if err = s.loadCalendarMetrics(ctx, &result, now); err != nil {
		return result, err
	}
	customerSource, err := s.source(ctx, DataQuery{Metric: MetricCustomers, EndAt: result.AsOf})
	if err != nil {
		return result, err
	}
	online, err := s.onlineCustomerPredicate(ctx)
	if err != nil {
		return result, err
	}
	customer, err := s.database.GetOne(ctx, "SELECT COUNT(*) total,COALESCE(SUM(CASE WHEN r.created_at>=? THEN 1 ELSE 0 END),0) today,COALESCE(SUM(CASE WHEN "+online+" THEN 1 ELSE 0 END),0) online FROM "+customerSource.from+" WHERE "+customerSource.where, append([]any{time.UnixMilli(result.Periods[0].StartAt)}, customerSource.args...)...)
	if err != nil {
		return result, err
	}
	result.Customers = DataCustomers{Today: customer["today"].Int64(), Total: customer["total"].Int64(), Online: customer["online"].Int64()}
	serverSource, err := s.source(ctx, DataQuery{Metric: MetricServers, StartAt: result.AsOf, EndAt: now.AddDate(0, 0, 7).UnixMilli()})
	if err != nil {
		return result, err
	}
	serverCount, err := s.database.GetValue(ctx, "SELECT COUNT(*) FROM "+serverSource.from+" WHERE "+serverSource.where, serverSource.args...)
	if err != nil {
		return result, err
	}
	result.ExpiringServers = serverCount.Int64()
	tickets, err := s.database.GetOne(ctx, `SELECT COALESCE(SUM(CASE WHEN base_stage IN ('resolved','closed') THEN 1 ELSE 0 END),0) processed,COALESCE(SUM(CASE WHEN base_stage NOT IN ('resolved','closed') THEN 1 ELSE 0 END),0) pending FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND created_at<?`, now)
	if err != nil {
		return result, err
	}
	result.Tickets = DataTickets{Pending: tickets["pending"].Int64(), Processed: tickets["processed"].Int64()}
	products, err := s.source(ctx, DataQuery{Metric: MetricProducts, StartAt: result.Periods[2].StartAt, EndAt: result.AsOf, CurrencyCode: code})
	if err != nil {
		return result, err
	}
	rows, err := s.database.GetAll(ctx, "SELECT r.product_no,MAX(r.product_name_zh_cn) name_zh_cn,MAX(r.product_name_en_us) name_en_us,SUM("+products.quantity+") quantity FROM "+products.from+" WHERE "+products.where+" GROUP BY r.product_no ORDER BY quantity DESC,r.product_no LIMIT 10", products.args...)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Products = append(result.Products, DataProduct{ProductNo: row["product_no"].String(), NameZhCN: row["name_zh_cn"].String(), NameEnUS: row["name_en_us"].String(), Quantity: row["quantity"].Int64()})
	}
	return result, nil
}

// loadPeriodMetrics performs two aggregate queries for all eight headline values.
func (s *serviceImpl) loadPeriodMetrics(ctx context.Context, result *Data) error {
	for _, metric := range []DataMetric{MetricIncome, MetricOrders} {
		src, err := s.source(ctx, DataQuery{Metric: metric, CurrencyCode: result.CurrencyCode, EndAt: result.AsOf})
		if err != nil {
			return err
		}
		var (
			fields []string
			args   []any
		)
		amount := "1"
		if metric == MetricIncome {
			amount = src.amount
		}
		for i, p := range result.Periods {
			predicate := src.event + "<?"
			args = append(args, time.UnixMilli(p.EndAt))
			if p.StartAt > 0 {
				predicate += " AND " + src.event + ">=?"
				args = append(args, time.UnixMilli(p.StartAt))
			}
			fields = append(fields, "COALESCE(SUM(CASE WHEN "+predicate+" THEN "+amount+" ELSE 0 END),0) p"+strconv.Itoa(i))
		}
		args = append(args, src.args...)
		row, err := s.database.GetOne(ctx, "SELECT "+strings.Join(fields, ",")+" FROM "+src.from+" WHERE "+src.where, args...)
		if err != nil {
			return err
		}
		for i := range result.Periods {
			v := row["p"+strconv.Itoa(i)]
			if metric == MetricIncome {
				result.Periods[i].IncomeMinor = v.String()
			} else {
				result.Periods[i].PaidOrders = v.Int64()
			}
		}
	}
	return nil
}

// loadCalendarMetrics uses fixed calendar predicates in two aggregate queries.
// Explicit Go time boundaries also support daylight-saving days without relying
// on a connection timezone or a database-specific date-truncation function.
func (s *serviceImpl) loadCalendarMetrics(ctx context.Context, result *Data, now time.Time) error {
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for _, metric := range []DataMetric{MetricIncome, MetricOrders} {
		src, err := s.source(ctx, DataQuery{Metric: metric, CurrencyCode: result.CurrencyCode, StartAt: month.AddDate(0, -1, 0).UnixMilli(), EndAt: result.AsOf})
		if err != nil {
			return err
		}
		var (
			fields []string
			args   []any
		)
		amount := "1"
		if metric == MetricIncome {
			amount = src.amount
		}
		add := func(alias string, from, to int64) {
			fields = append(fields, "COALESCE(SUM(CASE WHEN "+src.event+">=? AND "+src.event+"<? THEN "+amount+" ELSE 0 END),0) "+alias)
			args = append(args, time.UnixMilli(from), time.UnixMilli(to))
		}
		for i, day := range result.Trend {
			add("c"+strconv.Itoa(i), day.StartAt, day.EndAt)
			if metric == MetricOrders && day.PreviousCount != nil {
				add("p"+strconv.Itoa(i), day.PreviousStartAt, day.PreviousEndAt)
			}
		}
		args = append(args, src.args...)
		row, err := s.database.GetOne(ctx, "SELECT "+strings.Join(fields, ",")+" FROM "+src.from+" WHERE "+src.where, args...)
		if err != nil {
			return err
		}
		for i := range result.Trend {
			day := &result.Trend[i]
			if metric == MetricIncome {
				day.CurrentMinor = row["c"+strconv.Itoa(i)].String()
			} else {
				day.CurrentCount = row["c"+strconv.Itoa(i)].Int64()
				if day.PreviousCount != nil {
					*day.PreviousCount = row["p"+strconv.Itoa(i)].Int64()
				}
			}
		}
	}
	return nil
}
