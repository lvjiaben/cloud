// This file tests operating drill-downs against an isolated PostgreSQL schema,
// including real cash rows, quantity snapshots, temporal filters and staff gates.

package operations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
)

// TestDataRecordsPostgreSQL demonstrates aggregate sources and visible records
// count the same facts while rejecting future, tenant, and audience leakage.
func TestDataRecordsPostgreSQL(t *testing.T) {
	var (
		ctx      = context.Background()
		database = newDataQueryDatabase(t)
		current  = bizctxcap.CurrentContext{UserID: 1, TenantID: 0, DataScope: 2}
		svc      = &serviceImpl{database: database, bizCtx: operationsBizCtxStub{current: current}, authorization: operationsAuthorizationFixture{allowed: true}}
	)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(location).AddDate(0, 0, -1)
	asOf := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, location)
	paidAt := asOf.Add(-time.Hour)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO plugin_idc_core_merchant_profile VALUES (0,true,'Asia/Shanghai')`)
	exec(`INSERT INTO plugin_idc_core_principal_audience VALUES (0,1,'staff','active')`)
	for _, row := range []struct {
		no, typ, direction, role, currency string
		tenant                             int
		amount                             int64
		at                                 time.Time
	}{
		{"receipt", "payment", "debit", "cash", "CNY", 0, 500, paidAt},
		{"credit", "credit", "debit", "cash", "CNY", 0, 700, paidAt},
		{"balance", "balance_payment", "debit", "cash", "CNY", 0, 900, paidAt},
		{"migration", "migration", "debit", "cash", "CNY", 0, 1100, paidAt},
		{"refund", "refund", "credit", "cash", "CNY", 0, 200, paidAt},
		{"other-role", "payment", "debit", "receivable", "CNY", 0, 1400, paidAt},
		{"foreign", "payment", "debit", "cash", "CNY", 7, 1600, paidAt},
		{"usd", "payment", "debit", "cash", "USD", 0, 1800, paidAt},
		{"future", "payment", "debit", "cash", "CNY", 0, 2000, asOf.Add(time.Hour)},
	} {
		exec(`INSERT INTO plugin_idc_core_billing_ledger_transaction VALUES (?,?,?,?,?,?)`, row.tenant, row.no, row.no, row.currency, row.typ, row.at)
		exec(`INSERT INTO plugin_idc_core_billing_ledger_entry VALUES (?,?,?,?,?,?)`, row.tenant, row.no, row.currency, row.amount, row.role, row.direction)
	}
	exec(`INSERT INTO plugin_idc_core_customer_subject VALUES (0,'C1','Today','active',?),(0,'C2','Yesterday','active',?),(0,'C3','Future','active',?)`, paidAt, asOf.AddDate(0, 0, -1), asOf.Add(time.Hour))
	exec(`INSERT INTO plugin_idc_core_commerce_order VALUES (0,'O1','C1',8000,'CNY','paid',?),(0,'O2','C1',9000,'CNY','pending_payment',NULL),(0,'O3','C1',7000,'CNY','paid',?)`, paidAt, asOf.Add(time.Hour))
	exec(`INSERT INTO plugin_idc_core_commerce_order_item VALUES (0,'I1','O1','P1','商品','Product',8000,'CNY')`)
	exec(`INSERT INTO plugin_idc_core_commerce_order_item_quantity_snapshot VALUES (0,'I1',7)`)
	exec(`INSERT INTO plugin_idc_core_service_instance VALUES
 (0,'S1','Due','C1','idc-resource','cloud_server','active',?),
 (0,'S2','Expired','C1','idc-resource','cloud_server','active',?),
 (0,'S3','Beyond','C1','idc-resource','bare_metal','active',?),
 (0,'S4','Other product','C1','idc-resource','license','active',?),
 (0,'S5','Dedicated','C1','idc-resource','bare_metal','active',?)`, asOf.AddDate(0, 0, 2), asOf.Add(-time.Hour), asOf.AddDate(0, 0, 8), asOf.AddDate(0, 0, 2), asOf.AddDate(0, 0, 6))
	exec(`INSERT INTO plugin_idc_core_support_ticket VALUES (0,'T1','Open','C1','open',?),(0,'T2','Resolved','C1','resolved',?),(0,'T3','Closed','C1','closed',?),(0,'T4','Future','C1','open',?)`, paidAt, paidAt, paidAt, asOf.Add(time.Hour))
	for _, tc := range []struct {
		metric DataMetric
		status string
		total  int64
	}{
		{MetricIncome, "", 2}, {MetricOrders, "", 1}, {MetricProducts, "", 1}, {MetricCustomers, "today", 1}, {MetricCustomers, "", 2}, {MetricServers, "", 2}, {MetricTickets, "pending", 1}, {MetricTickets, "processed", 2},
	} {
		t.Run(string(tc.metric)+tc.status, func(t *testing.T) {
			q := DataQuery{Metric: tc.metric, Status: tc.status, AsOf: asOf.UnixMilli(), CurrencyCode: "CNY", Page: 1, PageSize: 20}
			out, err := svc.ListDataRecords(ctx, q)
			if err != nil || out.Total != tc.total || int64(len(out.Items)) != tc.total {
				t.Fatalf("total=%d items=%d expected=%d err=%v", out.Total, len(out.Items), tc.total, err)
			}
			if tc.metric == MetricIncome {
				if out.Items[0].AmountMinor != "500" || out.Items[1].AmountMinor != "700" {
					t.Fatalf("cash debit records wrong: %+v", out.Items)
				}
			}
			if tc.metric == MetricProducts && out.Items[0].Quantity != 7 {
				t.Fatalf("quantity snapshot lost: %+v", out.Items)
			}
		})
	}
	q := DataQuery{Metric: MetricOrders, CurrencyCode: "CNY", AsOf: asOf.UnixMilli(), Page: 1, PageSize: 20}
	svc.authorization = operationsAuthorizationFixture{err: errors.New("host authority unavailable")}
	if _, err = svc.ListDataRecords(ctx, q); !bizerr.Is(err, CodeDataUnavailable) {
		t.Fatalf("host authorization failure was not closed: %v", err)
	}
	svc.authorization = operationsAuthorizationFixture{allowed: true}
	exec(`UPDATE plugin_idc_core_principal_audience SET audience_code='customer' WHERE sys_user_id=1`)
	if _, err = svc.ListDataRecords(ctx, q); !bizerr.Is(err, CodeDataDenied) {
		t.Fatalf("customer with broad host role accessed records: %v", err)
	}
	exec(`UPDATE plugin_idc_core_principal_audience SET audience_code='staff' WHERE sys_user_id=1`)
	exec(`INSERT INTO plugin_idc_core_customer_identity_binding VALUES (0,1,'C1','active')`)
	if _, err = svc.ListDataRecords(ctx, q); !bizerr.Is(err, CodeDataDenied) {
		t.Fatalf("conflicting customer binding accessed records: %v", err)
	}
	exec(`DELETE FROM plugin_idc_core_customer_identity_binding`)
	svc.authorization = operationsAuthorizationFixture{}
	current.IsSuperAdmin = false
	svc.bizCtx = operationsBizCtxStub{current: current}
	if _, err = svc.ListDataRecords(ctx, q); !bizerr.Is(err, CodeDataDenied) {
		t.Fatalf("aggregate scope substituted for domain permission: %v", err)
	}
	q.Metric, q.Status = MetricTickets, "pending"
	current.Permissions = []string{"idc:admin:support:ticket:list"}
	svc.bizCtx = operationsBizCtxStub{current: current}
	if _, err = svc.ListDataRecords(ctx, q); !bizerr.Is(err, CodeDataDenied) {
		t.Fatalf("own-ticket list permission exposed all tickets: %v", err)
	}
	svc.authorization = operationsAuthorizationFixture{allowed: true}
	current.Permissions = []string{"idc:admin:support:ticket:view-all"}
	svc.bizCtx = operationsBizCtxStub{current: current}
	if out, err := svc.ListDataRecords(ctx, q); err != nil || out.Total != 1 {
		t.Fatalf("view-all did not return scoped tickets: total=%d err=%v", out.Total, err)
	}
}

// newDataQueryDatabase provisions the smallest isolated schema for executing
// the production source queries; cleanup is registered before any fixture DDL.
func newDataQueryDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run PostgreSQL operating-data tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(ctx); err != nil {
			t.Errorf("close admin: %v", err)
		}
	})
	var token [8]byte
	if _, err = rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	schema := "operations_data_" + hex.EncodeToString(token[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(ctx); err != nil {
			t.Errorf("close test db: %v", err)
		}
	})
	for _, ddl := range []string{
		`CREATE TABLE plugin_idc_core_merchant_profile(tenant_id INT,enabled BOOL,timezone TEXT)`,
		`CREATE TABLE plugin_idc_core_principal_audience(tenant_id INT,sys_user_id INT,audience_code TEXT,status_code TEXT)`,
		`CREATE TABLE plugin_idc_core_customer_identity_binding(tenant_id INT,sys_user_id INT,customer_no TEXT,status_code TEXT)`,
		`CREATE TABLE plugin_idc_core_customer_subject(tenant_id INT,customer_no TEXT,display_name TEXT,status_code TEXT,created_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_billing_ledger_transaction(tenant_id INT,transaction_no TEXT,source_reference TEXT,currency_code TEXT,transaction_type_code TEXT,posted_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_billing_ledger_entry(tenant_id INT,transaction_no TEXT,currency_code TEXT,amount_minor BIGINT,entry_role_code TEXT,direction_code TEXT)`,
		`CREATE TABLE plugin_idc_core_commerce_order(tenant_id INT,order_no TEXT,customer_no TEXT,total_amount_minor BIGINT,currency_code TEXT,status_code TEXT,paid_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_commerce_order_item(tenant_id INT,order_item_no TEXT,order_no TEXT,product_no TEXT,product_name_zh_cn TEXT,product_name_en_us TEXT,total_amount_minor BIGINT,currency_code TEXT)`,
		`CREATE TABLE plugin_idc_core_commerce_order_item_quantity_snapshot(tenant_id INT,order_item_no TEXT,quantity INT)`,
		`CREATE TABLE plugin_idc_core_service_instance(tenant_id INT,service_no TEXT,customer_alias TEXT,customer_no TEXT,resource_owner_code TEXT,product_type_code TEXT,entitlement_status_code TEXT,entitlement_expires_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_support_ticket(tenant_id INT,ticket_no TEXT,subject TEXT,customer_no TEXT,base_stage TEXT,created_at TIMESTAMPTZ)`,
	} {
		if _, err = database.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	return database
}
