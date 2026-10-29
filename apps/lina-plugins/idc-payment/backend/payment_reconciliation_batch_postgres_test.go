// This file verifies bounded reconciliation writes with real migrations,
// idempotent replay and rollback after a later batch fails. No money is moved.
package backend

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
	"lina-plugin-idc-payment/backend/internal/service/reconciliation"
)

type reconciliationBatchDatabase struct {
	gdb.DB
	sourceWrites, lineWrites int
}

func (d *reconciliationBatchDatabase) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if strings.HasPrefix(query, "INSERT INTO plugin_idc_payment_reconciliation_source") {
		d.sourceWrites++
	}
	if strings.HasPrefix(query, "INSERT INTO plugin_idc_payment_reconciliation_line") {
		d.lineWrites++
	}
	return d.DB.Exec(ctx, query, args...)
}

type reconciliationBatchLoader struct{ records []reconciliation.SourceRecord }

func (l reconciliationBatchLoader) Load(context.Context, string, time.Time, int) ([]reconciliation.SourceRecord, string, error) {
	return l.records, strings.Repeat("e", 64), nil
}

func TestReconciliationBatchesAreAtomicPostgreSQL(t *testing.T) {
	database := newPaymentPagesDatabase(t)
	ctx := t.Context()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_payment_channel(channel_no,provider_code,display_name,endpoint_origin,merchant_reference,secret_ref,supported_modes,supported_currencies,capability_snapshot,status_code,verified_at) VALUES ('batch-channel','epay','Batch channel','https://payment.example.test','merchant','synthetic-secret','["epay_cashier"]','["CNY"]','{"statement":true}','enabled',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	records := make([]reconciliation.SourceRecord, 1000)
	for index := range records {
		records[index] = reconciliation.SourceRecord{RecordType: "payment", LocalSubjectNo: fmt.Sprintf("payment-%04d", index), ProviderEventReference: fmt.Sprintf("event-%04d", index), ExternalReference: fmt.Sprintf("trade-%04d", index), CurrencyCode: "CNY", AmountMinor: int64(index + 1), StatusCode: "succeeded", EvidenceDigest: strings.Repeat("a", 64), ObservedAt: day}
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_payment_attempt(attempt_no,channel_no,invoice_no,order_no,customer_ref_digest,currency_code,amount_minor,mode_code,request_digest,expires_at,status_code,external_trade_reference,created_at) VALUES ('payment-0000','batch-channel','fixture-invoice','fixture-order',repeat('b',64),'CNY',1,'epay_cashier',repeat('c',64),CURRENT_TIMESTAMP+INTERVAL '1 hour','succeeded','trade-0000',?)`, day); err != nil {
		t.Fatal(err)
	}
	counted := &reconciliationBatchDatabase{DB: database}
	// RunChannel reads capabilities from the database and only uses the supplied
	// loader; these unused command dependencies must not contact any provider.
	service, err := reconciliation.New(counted, &channelconfig.Service{}, &paymentattempt.Store{}, reconciliationBatchLoader{records})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.RunChannel(ctx, "batch-channel", day, "UTC", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if first.TotalCount != 1000 || first.DifferenceCount != 999 || first.StatusCode != "differences" || counted.sourceWrites != 2 || counted.lineWrites != 2 {
		t.Fatalf("unexpected batch: %+v, writes=%d/%d", first, counted.sourceWrites, counted.lineWrites)
	}
	total, err := database.GetValue(ctx, `SELECT sum(amount_minor) FROM plugin_idc_payment_reconciliation_source WHERE batch_no=?`, first.BatchNo)
	if err != nil || total.Int64() != 500500 {
		t.Fatalf("amounts changed: %v", err)
	}
	digest, err := database.GetValue(ctx, `SELECT customer_ref_digest FROM plugin_idc_payment_reconciliation_source WHERE batch_no=? AND local_subject_no='payment-0000'`, first.BatchNo)
	if err != nil || digest.String() != strings.Repeat("b", 64) {
		t.Fatalf("local customer attribution changed: %v", err)
	}
	matched, err := database.GetValue(ctx, `SELECT difference_code FROM plugin_idc_payment_reconciliation_line WHERE batch_no=? AND attempt_no='payment-0000'`, first.BatchNo)
	if err != nil || matched.String() != "none" {
		t.Fatalf("matched payment was reclassified: %v", err)
	}
	replay, err := service.RunChannel(ctx, "batch-channel", day, "UTC", 1000)
	if err != nil || replay.BatchNo != first.BatchNo || counted.sourceWrites != 2 || counted.lineWrites != 2 {
		t.Fatalf("replay wrote duplicate evidence: %v", err)
	}
	if _, err = database.Exec(ctx, `CREATE FUNCTION reject_late_reconciliation_batch() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.external_reference='trade-0750' THEN RAISE EXCEPTION 'fixture failure'; END IF; RETURN NEW; END; $$; CREATE TRIGGER reject_late_reconciliation_batch BEFORE INSERT ON plugin_idc_payment_reconciliation_line FOR EACH ROW EXECUTE FUNCTION reject_late_reconciliation_batch()`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RunChannel(ctx, "batch-channel", day.AddDate(0, 0, 1), "UTC", 1000); err == nil {
		t.Fatal("second line batch failure was ignored")
	}
	for table, want := range map[string]int{"plugin_idc_payment_reconciliation_batch": 1, "plugin_idc_payment_reconciliation_source": 1000, "plugin_idc_payment_reconciliation_line": 1000} {
		count, queryErr := database.GetValue(ctx, "SELECT count(*) FROM "+table)
		if queryErr != nil || count.Int() != want {
			t.Fatalf("partial %s remains: %v count=%d", table, queryErr, count.Int())
		}
	}
	health, err := database.GetValue(ctx, `SELECT latest_batch_no FROM plugin_idc_payment_reconciliation_channel_health WHERE channel_no='batch-channel'`)
	if err != nil || health.String() != first.BatchNo {
		t.Fatalf("failed batch changed channel health: %v", err)
	}
}
