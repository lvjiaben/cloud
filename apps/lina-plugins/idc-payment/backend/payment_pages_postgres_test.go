// This file verifies payment pagination against real migrations in an isolated schema.
// No supplier, existing account, payment, or refund operation is executed.
package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/provider/epay"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/exceptionfund"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
	"lina-plugin-idc-payment/backend/internal/service/reconciliation"
)

func newPaymentPagesDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(paymentTestDSN))
	if dsn == "" {
		t.Skip("payment PostgreSQL DSN is not configured")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_payment_pages_" + hex.EncodeToString(nonce[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quotePaymentIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "linapro_payment_pages_") {
			t.Fatal("unsafe test schema")
		}
		if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quotePaymentIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Error(dropErr)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendPaymentDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(ctx) })
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migrations")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) != 5 {
		t.Fatalf("expected five payment migrations: %v", err)
	}
	for pass := 0; pass < 2; pass++ {
		for _, path := range paths {
			sql, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if _, err = db.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("migration %s: %v", filepath.Base(path), err)
			}
		}
	}
	return db
}

func seedPaymentPages(t *testing.T, db gdb.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO plugin_idc_payment_channel(channel_no,provider_code,display_name,endpoint_origin,merchant_reference,secret_ref,supported_modes,supported_currencies,capability_snapshot,status_code)
 SELECT 'channel-'||i,'epay','Channel '||lpad(i::text,3,'0'),'https://payment.example.test','merchant-'||i,'synthetic-secret-'||i,'["epay_cashier"]','["CNY"]','{}','disabled' FROM generate_series(1,122) i`,
		`UPDATE plugin_idc_payment_channel SET status_code='deleted',deleted_at=CURRENT_TIMESTAMP WHERE channel_no='channel-122'`,
		`INSERT INTO plugin_idc_payment_reconciliation_batch(batch_no,channel_no,business_date,source_digest)
 SELECT 'batch-'||i,CASE WHEN i=122 THEN 'channel-2' ELSE 'channel-1' END,DATE '2026-01-01'+i,md5(i::text)||md5(i::text) FROM generate_series(1,122) i`,
		`INSERT INTO plugin_idc_payment_reconciliation_line(line_no,batch_no,external_reference,difference_code,currency_code,evidence_digest)
 SELECT 'line-'||i,CASE WHEN i=523 THEN 'batch-2' ELSE 'batch-1' END,'external-'||i,CASE WHEN i=522 THEN 'none' ELSE 'amount_mismatch' END,'CNY',repeat('a',64) FROM generate_series(1,523) i`,
		`INSERT INTO plugin_idc_payment_attempt(attempt_no,channel_no,invoice_no,order_no,customer_ref_digest,currency_code,amount_minor,mode_code,request_digest,expires_at)
 SELECT 'attempt-'||i,'channel-1','invoice-'||i,'order-'||i,repeat('a',64),'CNY',100,'epay_cashier',md5(i::text)||md5(i::text),CURRENT_TIMESTAMP+INTERVAL '1 hour' FROM generate_series(1,124) i`,
		`INSERT INTO plugin_idc_payment_exception_fund(exception_no,attempt_no,channel_no,currency_code,amount_minor,reason_code,status_code,evidence_digest)
 SELECT 'fund-'||i,'attempt-'||i,'channel-1','CNY',100,'late_success',CASE WHEN i>121 THEN 'closed' WHEN i%2=0 THEN 'disputed' ELSE 'pending' END,repeat('a',64) FROM generate_series(1,124) i`,
	}
	for _, sql := range statements {
		if _, err := db.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPaymentNumericPagesPostgreSQL(t *testing.T) {
	db := newPaymentPagesDatabase(t)
	seedPaymentPages(t, db)
	ctx := context.Background()
	secret := &paymentSecretFake{}
	adapter, err := epay.NewWithClient("", &http.Client{Transport: paymentRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("pagination must not contact a supplier")
	})})
	if err != nil {
		t.Fatal(err)
	}
	channels, err := channelconfig.New(db, secret, secret, []paymentcap.Adapter{adapter}, func(context.Context, paymentcap.ProviderCode, paymentcap.Credentials) error {
		return fmt.Errorf("pagination must not verify credentials")
	})
	if err != nil {
		t.Fatal(err)
	}
	billing := &paymentBillingFake{}
	store, err := paymentattempt.New(db, billing)
	if err != nil {
		t.Fatal(err)
	}
	reconcile, err := reconciliation.New(db, channels, store, paymentReconciliationLoaderFake{})
	if err != nil {
		t.Fatal(err)
	}
	funds, err := exceptionfund.New(db, billing)
	if err != nil {
		t.Fatal(err)
	}
	readers := []struct {
		name  string
		total int
		read  func(int, int) (int, []string, error)
	}{
		{"configured channels", 121, func(page, size int) (int, []string, error) {
			result, e := channels.ListManaged(ctx, page, size)
			if e != nil {
				return 0, nil, e
			}
			ids := []string{}
			for _, item := range result.Items {
				ids = append(ids, item.ChannelNo)
			}
			return result.Total, ids, nil
		}},
		{"channel health", 121, func(page, size int) (int, []string, error) {
			result, e := reconcile.ListHealth(ctx, page, size)
			if e != nil {
				return 0, nil, e
			}
			ids := []string{}
			for _, item := range result.Items {
				ids = append(ids, item.ChannelNo)
			}
			return result.Total, ids, nil
		}},
		{"all batches", 122, func(page, size int) (int, []string, error) {
			result, e := reconcile.ListBatches(ctx, "", page, size)
			if e != nil {
				return 0, nil, e
			}
			ids := []string{}
			for _, item := range result.Items {
				ids = append(ids, item.BatchNo)
			}
			return result.Total, ids, nil
		}},
		{"filtered batches", 121, func(page, size int) (int, []string, error) {
			result, e := reconcile.ListBatches(ctx, "channel-1", page, size)
			if e != nil {
				return 0, nil, e
			}
			ids := []string{}
			for _, item := range result.Items {
				if item.ChannelNo != "channel-1" {
					t.Fatal("cross-channel batch")
				}
				ids = append(ids, item.BatchNo)
			}
			return result.Total, ids, nil
		}},
		{"differences beyond 500", 521, func(page, size int) (int, []string, error) {
			result, e := reconcile.ListDifferences(ctx, "batch-1", page, size)
			if e != nil {
				return 0, nil, e
			}
			ids := []string{}
			for _, item := range result.Items {
				if item.BatchNo != "batch-1" || item.DifferenceCode == "none" {
					t.Fatal("wrong difference scope")
				}
				ids = append(ids, item.LineNo)
			}
			return result.Total, ids, nil
		}},
		{"pending and disputed funds", 121, func(page, size int) (int, []string, error) {
			result, e := funds.List(ctx, page, size)
			if e != nil {
				return 0, nil, e
			}
			ids := []string{}
			for _, item := range result.Items {
				if item.StatusCode != "pending" && item.StatusCode != "disputed" {
					t.Fatal("closed fund leaked")
				}
				ids = append(ids, item.ExceptionNo)
			}
			return result.Total, ids, nil
		}},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			for _, size := range []int{10, 20, 50, 100} {
				seen := map[string]bool{}
				for page := 1; page <= (reader.total+size-1)/size+1; page++ {
					total, ids, e := reader.read(page, size)
					if e != nil || total != reader.total {
						t.Fatalf("page %d/%d total=%d error=%v", page, size, total, e)
					}
					expected := min(size, max(0, reader.total-(page-1)*size))
					if len(ids) != expected {
						t.Fatalf("rows=%d want=%d", len(ids), expected)
					}
					for _, id := range ids {
						if seen[id] {
							t.Fatal("duplicate page row", id)
						}
						seen[id] = true
					}
				}
				if len(seen) != reader.total {
					t.Fatal("lost rows", len(seen))
				}
			}
			for _, input := range [][2]int{{0, 20}, {1, 0}, {1, 101}, {1_000_001, 20}} {
				if _, _, e := reader.read(input[0], input[1]); e == nil {
					t.Fatal("invalid page accepted", input)
				}
			}
		})
	}
	empty, err := reconcile.ListBatches(ctx, "channel-missing", 1, 20)
	if err != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatal("empty filter", err)
	}
	missing, err := reconcile.ListDifferences(ctx, "batch-missing", 1, 20)
	if err != nil || missing.Total != 0 || len(missing.Items) != 0 {
		t.Fatal("empty batch", err)
	}
	for _, input := range []string{"channel-1' OR 1=1 --", "batch-1' OR 1=1 --"} {
		batchPage, batchErr := reconcile.ListBatches(ctx, input, 1, 20)
		differencePage, differenceErr := reconcile.ListDifferences(ctx, input, 1, 20)
		if batchErr != nil || differenceErr != nil || batchPage.Total != 0 || differencePage.Total != 0 {
			t.Fatal("exact identifier scope was expanded", batchErr, differenceErr)
		}
	}
	// A deleted channel leaves historical funds actionable, but must disappear
	// from current channel configuration and health directory counts and rows.
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_payment_channel SET status_code='deleted',deleted_at=CURRENT_TIMESTAMP WHERE channel_no='channel-1'`); err != nil {
		t.Fatal(err)
	}
	channelPage, channelErr := channels.ListManaged(ctx, 1, 20)
	healthPage, healthErr := reconcile.ListHealth(ctx, 1, 20)
	fundPage, fundErr := funds.List(ctx, 1, 20)
	if channelErr != nil || healthErr != nil || fundErr != nil || channelPage.Total != 120 || healthPage.Total != 120 || fundPage.Total != 121 || len(fundPage.Items) != 20 {
		t.Fatal("channel deletion changed historical fund or visible-directory scope", channelErr, healthErr, fundErr)
	}
	for _, item := range fundPage.Items {
		if item.ProviderCode != "epay" || item.ChannelNo != "channel-1" {
			t.Fatal("historical fund metadata was lost")
		}
	}
}

func TestPaymentDirectoryPageIndexPlansPostgreSQL(t *testing.T) {
	testDB := newPaymentPagesDatabase(t)
	seedPaymentPages(t, testDB)
	ctx := context.Background()
	seed := []string{
		`INSERT INTO plugin_idc_payment_channel(channel_no,provider_code,display_name,endpoint_origin,merchant_reference,secret_ref,supported_modes,supported_currencies,capability_snapshot,status_code)
SELECT 'extra-channel-'||i,'epay','Extra '||i,'https://payment.example.test','extra-'||i,'synthetic-'||i,'["epay_cashier"]','["CNY"]','{}','disabled' FROM generate_series(1,20000) i`,
		`INSERT INTO plugin_idc_payment_reconciliation_batch(batch_no,channel_no,business_date,source_digest)
SELECT 'extra-batch-'||i,'extra-channel-'||i,DATE '2026-01-01',md5(i::text)||md5(i::text) FROM generate_series(1,20000) i`,
		`INSERT INTO plugin_idc_payment_reconciliation_line(line_no,batch_no,external_reference,difference_code,currency_code,evidence_digest)
SELECT 'extra-line-'||i,'extra-batch-'||i,'extra-external-'||i,'amount_mismatch','CNY',repeat('a',64) FROM generate_series(1,20000) i`,
		`INSERT INTO plugin_idc_payment_attempt(attempt_no,channel_no,invoice_no,order_no,customer_ref_digest,currency_code,amount_minor,mode_code,request_digest,expires_at)
SELECT 'extra-attempt-'||i,'channel-1','extra-invoice-'||i,'extra-order-'||i,repeat('a',64),'CNY',100,'epay_cashier',md5(i::text)||md5(i::text),CURRENT_TIMESTAMP+INTERVAL '1 hour' FROM generate_series(1,20000) i`,
		`INSERT INTO plugin_idc_payment_exception_fund(exception_no,attempt_no,channel_no,currency_code,amount_minor,reason_code,status_code,evidence_digest)
SELECT 'extra-fund-'||i,'extra-attempt-'||i,'channel-1','CNY',100,'late_success','closed',repeat('a',64) FROM generate_series(1,20000) i`,
		`ANALYZE plugin_idc_payment_channel; ANALYZE plugin_idc_payment_reconciliation_batch; ANALYZE plugin_idc_payment_reconciliation_line; ANALYZE plugin_idc_payment_exception_fund`,
	}
	for _, sql := range seed {
		if _, err := testDB.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	plans := []struct{ index, query string }{
		{"idx_idc_payment_channel_health_page", `SELECT channel_no,display_name FROM plugin_idc_payment_channel WHERE tenant_id=0 AND deleted_at IS NULL ORDER BY display_name,channel_no LIMIT 20`},
		{"idx_idc_payment_batch_channel_page", `SELECT batch_no FROM plugin_idc_payment_reconciliation_batch WHERE channel_no='channel-1' ORDER BY business_date DESC,id DESC LIMIT 20`},
		{"idx_idc_payment_batch_all_page", `SELECT batch_no FROM plugin_idc_payment_reconciliation_batch ORDER BY business_date DESC,id DESC LIMIT 20`},
		{"idx_idc_payment_difference_page", `SELECT line_no FROM plugin_idc_payment_reconciliation_line WHERE batch_no='extra-batch-10000' AND difference_code<>'none' ORDER BY id LIMIT 20`},
		{"idx_idc_payment_exception_pending_page", `SELECT exception_no FROM plugin_idc_payment_exception_fund WHERE status_code IN ('pending','disputed') ORDER BY created_at,id LIMIT 20`},
	}
	for _, plan := range plans {
		rows, err := testDB.GetAll(ctx, "EXPLAIN (FORMAT JSON) "+plan.query)
		if err != nil || !strings.Contains(fmt.Sprint(rows), plan.index) {
			t.Fatalf("index %s not used: %v %v", plan.index, rows, err)
		}
	}
}
