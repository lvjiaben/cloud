// This file exercises the full operating-data aggregate and compares every
// headline and chart drill-down with the records produced from the same facts.

package operations

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
)

// TestLoadDataPostgreSQL verifies the complete aggregation path rather than
// merely testing its source fragments. Session presence stays behind an SPI.
func TestLoadDataPostgreSQL(t *testing.T) {
	var (
		ctx      = context.Background()
		database = newDataQueryDatabase(t)
		svc      = &serviceImpl{database: database, bizCtx: operationsBizCtxStub{current: bizctxcap.CurrentContext{UserID: 1, DataScope: 2, IsSuperAdmin: true}}, sessions: operationsPresenceFixture{database: database}, authorization: operationsAuthorizationFixture{allowed: true}}
	)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(location)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	today := day.Add(now.Sub(day) / 2)
	yesterday := day.Add(-30 * time.Minute)
	previous := month.AddDate(0, -1, 0).Add(30 * time.Minute)
	old := month.AddDate(0, -2, 0).Add(time.Hour)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`ALTER TABLE plugin_idc_core_merchant_profile ADD COLUMN default_currency TEXT`)
	exec(`CREATE TABLE test_session_presence(user_id INT,expires_at TIMESTAMPTZ)`)
	exec(`INSERT INTO plugin_idc_core_merchant_profile VALUES (0,true,'Asia/Shanghai','CNY')`)
	exec(`INSERT INTO plugin_idc_core_principal_audience VALUES (0,1,'staff','active'),(0,10,'customer','active'),(0,11,'customer','active'),(0,12,'customer','revoked'),(0,13,'staff','active')`)
	exec(`INSERT INTO plugin_idc_core_customer_subject VALUES (0,'C1','Today','active',?),(0,'C2','Existing','active',?),(0,'C3','Revoked identity','active',?),(0,'C4','Staff identity','active',?),(0,'CF','Future','active',?)`, today, old, old, old, now.AddDate(0, 0, 1))
	exec(`INSERT INTO plugin_idc_core_customer_identity_binding VALUES (0,10,'C1','active'),(0,11,'C1','active'),(0,11,'C2','revoked'),(0,12,'C3','active'),(0,13,'C4','active')`)
	exec(`INSERT INTO test_session_presence VALUES (10,?),(10,?),(11,?),(12,?),(13,?),(11,?)`, now.Add(time.Hour), now.Add(time.Hour), now.Add(time.Hour), now.Add(time.Hour), now.Add(time.Hour), now.Add(-time.Hour))
	times := []time.Time{today, yesterday, previous, old}
	amounts := []int64{100, 200, 400, 800}
	for i, at := range times {
		id := fmt.Sprintf("%d", i)
		exec(`INSERT INTO plugin_idc_core_billing_ledger_transaction VALUES (0,?,?,'CNY','payment',?)`, "TX"+id, "receipt"+id, at)
		exec(`INSERT INTO plugin_idc_core_billing_ledger_entry VALUES (0,?,'CNY',?,'cash','debit')`, "TX"+id, amounts[i])
		exec(`INSERT INTO plugin_idc_core_commerce_order VALUES (0,?,'C1',999999,'CNY','paid',?)`, "O"+id, at)
	}
	exec(`INSERT INTO plugin_idc_core_billing_ledger_transaction VALUES (0,'F','future','CNY','payment',?),(0,'U','usd','USD','payment',?)`, now.AddDate(0, 0, 1), today)
	exec(`INSERT INTO plugin_idc_core_billing_ledger_entry VALUES (0,'F','CNY',1600,'cash','debit'),(0,'U','USD',3200,'cash','debit')`)
	exec(`INSERT INTO plugin_idc_core_commerce_order VALUES (0,'OF','C1',999999,'CNY','paid',?),(0,'OU','C1',999999,'USD','paid',?),(0,'UNPAID','C1',999999,'CNY','pending',NULL)`, now.AddDate(0, 0, 1), today)
	exec(`INSERT INTO plugin_idc_core_commerce_order_item VALUES (0,'I1','O0','P1','商品一','Product one',700,'CNY'),(0,'I2','O0','P2','商品二','Product two',400,'CNY'),(0,'I3','O3','P2','商品二','Product two',500,'CNY')`)
	exec(`INSERT INTO plugin_idc_core_commerce_order_item_quantity_snapshot VALUES (0,'I1',7),(0,'I2',4),(0,'I3',50)`)
	exec(`INSERT INTO plugin_idc_core_service_instance VALUES (0,'S1','Due','C1','idc-resource','cloud_server','active',?),(0,'S2','Expired','C1','idc-resource','cloud_server','active',?),(0,'S3','Later','C1','idc-resource','bare_metal','active',?)`, now.AddDate(0, 0, 2), now.Add(-time.Hour), now.AddDate(0, 0, 8))
	exec(`INSERT INTO plugin_idc_core_support_ticket VALUES (0,'T1','Pending','C1','open',?),(0,'T2','Resolved','C1','resolved',?),(0,'T3','Closed','C1','closed',?)`, today, old, old)
	out, err := svc.LoadData(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.CurrencyCode != "CNY" || out.CurrencyExponent != 2 || out.Timezone != "Asia/Shanghai" {
		t.Fatalf("merchant configuration lost: %+v", out)
	}
	if len(out.Currencies) != 2 || out.Currencies[0] != "CNY" || out.Currencies[1] != "USD" {
		t.Fatalf("currency choices: %v", out.Currencies)
	}
	if out.Customers != (DataCustomers{Today: 1, Total: 4, Online: 1}) || out.ExpiringServers != 1 || out.Tickets != (DataTickets{Pending: 1, Processed: 2}) {
		t.Fatalf("customer/server/ticket aggregation incorrect: customers=%+v servers=%d tickets=%+v", out.Customers, out.ExpiringServers, out.Tickets)
	}
	if len(out.Products) != 2 || out.Products[0].ProductNo != "P1" || out.Products[0].Quantity != 7 || out.Products[1].ProductNo != "P2" || out.Products[1].Quantity != 4 {
		t.Fatalf("monthly quantity ranking: %+v", out.Products)
	}
	if len(out.Periods) != 4 {
		t.Fatalf("period count: %d", len(out.Periods))
	}
	for _, period := range out.Periods {
		var expectedAmount, expectedOrders int64
		for i, at := range times {
			if at.UnixMilli() >= period.StartAt && at.UnixMilli() < period.EndAt {
				expectedAmount += amounts[i]
				expectedOrders++
			}
		}
		if period.IncomeMinor != strconv.FormatInt(expectedAmount, 10) || period.PaidOrders != expectedOrders {
			t.Fatalf("period %s got income=%s orders=%d expected=%d/%d", period.Key, period.IncomeMinor, period.PaidOrders, expectedAmount, expectedOrders)
		}
		for _, metric := range []DataMetric{MetricIncome, MetricOrders} {
			rows, err := svc.ListDataRecords(ctx, DataQuery{Metric: metric, StartAt: period.StartAt, EndAt: period.EndAt, AsOf: out.AsOf, CurrencyCode: "CNY", Page: 1, PageSize: 100})
			if err != nil {
				t.Fatal(err)
			}
			if rows.Total != expectedOrders {
				t.Fatalf("period %s metric %s records=%d expected=%d", period.Key, metric, rows.Total, expectedOrders)
			}
			if metric == MetricIncome && sumDataRecordAmounts(t, rows.Items) != expectedAmount {
				t.Fatalf("period %s income drilldown disagrees", period.Key)
			}
		}
	}
	if out.Periods[0].IncomeMinor != "100" || out.Periods[1].IncomeMinor != "200" || out.Periods[3].IncomeMinor != "1500" {
		t.Fatalf("calendar or actual cash incorrect: %+v", out.Periods)
	}
	days := month.AddDate(0, 1, -1).Day()
	if len(out.Trend) != days {
		t.Fatalf("calendar days=%d expected=%d", len(out.Trend), days)
	}
	var monthIncome, monthOrders int64
	for _, point := range out.Trend {
		amount, err := strconv.ParseInt(point.CurrentMinor, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		monthIncome += amount
		monthOrders += point.CurrentCount
		q := DataQuery{Metric: MetricOrders, CurrencyCode: "CNY", StartAt: point.StartAt, EndAt: point.EndAt, AsOf: out.AsOf, Page: 1, PageSize: 100}
		rows, err := svc.ListDataRecords(ctx, q)
		if err != nil || rows.Total != point.CurrentCount {
			t.Fatalf("current day %s mismatch rows=%d count=%d err=%v", point.Date, rows.Total, point.CurrentCount, err)
		}
		q.Metric = MetricIncome
		incomeRows, err := svc.ListDataRecords(ctx, q)
		if err != nil || sumDataRecordAmounts(t, incomeRows.Items) != amount {
			t.Fatalf("current day %s income mismatch chart=%d rows=%+v err=%v", point.Date, amount, incomeRows.Items, err)
		}
		q.Metric = MetricOrders
		if point.PreviousCount != nil {
			q.StartAt, q.EndAt = point.PreviousStartAt, point.PreviousEndAt
			rows, err = svc.ListDataRecords(ctx, q)
			if err != nil || rows.Total != *point.PreviousCount {
				t.Fatalf("previous day %s mismatch rows=%d count=%d err=%v", point.PreviousDate, rows.Total, *point.PreviousCount, err)
			}
		}
	}
	if out.Periods[2].IncomeMinor != strconv.FormatInt(monthIncome, 10) || out.Periods[2].PaidOrders != monthOrders {
		t.Fatalf("month chart disagrees: income=%d orders=%d month=%+v", monthIncome, monthOrders, out.Periods[2])
	}
	for _, tc := range []struct {
		metric DataMetric
		status string
		count  int64
	}{{MetricCustomers, "today", 1}, {MetricCustomers, "", 4}, {MetricCustomers, "online", 1}, {MetricServers, "", 1}, {MetricTickets, "pending", 1}, {MetricTickets, "processed", 2}} {
		rows, err := svc.ListDataRecords(ctx, DataQuery{Metric: tc.metric, Status: tc.status, AsOf: out.AsOf, Page: 1, PageSize: 100})
		if err != nil || rows.Total != tc.count {
			t.Fatalf("%s/%s aggregate drilldown mismatch: %+v err=%v", tc.metric, tc.status, rows, err)
		}
	}
	for _, product := range out.Products {
		rows, err := svc.ListDataRecords(ctx, DataQuery{Metric: MetricProducts, ProductNo: product.ProductNo, CurrencyCode: "CNY", StartAt: out.Periods[2].StartAt, EndAt: out.AsOf, AsOf: out.AsOf, Page: 1, PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		var count int64
		for _, row := range rows.Items {
			count += row.Quantity
		}
		if count != product.Quantity {
			t.Fatalf("product rank disagrees: %s %d/%d", product.ProductNo, count, product.Quantity)
		}
	}
}

// operationsPresenceFixture implements only the host-owned SPI touched by this
// integration test; duplicate live rows model multiple sessions per identity.
type operationsPresenceFixture struct {
	sessioncap.Service
	database gdb.DB
}

// BuildOnlineUserExists returns a test-owned correlated valid-session model.
func (f operationsPresenceFixture) BuildOnlineUserExists(ctx context.Context, column string) (*gdb.Model, bool, error) {
	return f.database.Model("test_session_presence").Ctx(ctx).Fields(gdb.Raw("1")).Where("user_id", gdb.Raw(column)).WhereGT("expires_at", time.Now()), false, nil
}

// sumDataRecordAmounts checks exact integer income without floating conversion.
func sumDataRecordAmounts(t *testing.T, rows []DataRecord) int64 {
	t.Helper()
	var total int64
	for _, row := range rows {
		value, err := strconv.ParseInt(row.AmountMinor, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		total += value
	}
	return total
}
