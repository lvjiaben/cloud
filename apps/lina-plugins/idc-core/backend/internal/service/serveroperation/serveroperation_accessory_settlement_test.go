// This file tests addon evidence fencing, transactional recovery and idempotent
// delivery using isolated PostgreSQL fixtures without payment/provider writes.

package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

type accessorySettlementEvidenceStub struct {
	proof *billing.FinancialSettlement
	err   error
}

func (s accessorySettlementEvidenceStub) GetSettledOrderEvidence(context.Context, string) (*billing.FinancialSettlement, error) {
	return s.proof, s.err
}

func TestAccessorySettlementRejectsUnsupportedActionAndUnpaidProof(t *testing.T) {
	for _, test := range []struct {
		action, kind string
		valid        bool
	}{
		{"traffic_add", "traffic", true}, {"snapshot_create", "snapshot", true}, {"backup_create", "backup", true},
		{"snapshot_restore", "snapshot", false}, {"start", "traffic", false}, {"traffic_add", "snapshot", false},
	} {
		if accessorySettlementAction(test.action, test.kind) != test.valid {
			t.Fatalf("invalid action classification: %+v", test)
		}
	}
	if _, err := NewAccessorySettlementConsumer(nil, accessorySettlementEvidenceStub{}); err == nil {
		t.Fatal("nil database accepted")
	}
	if accessorySettlementEventMatches(fulfillment.SettlementEvent{}, nil) {
		t.Fatal("missing financial proof accepted")
	}
}

func TestAccessorySettlementConcurrentReplayPostgreSQL(t *testing.T) {
	for _, kind := range []string{"traffic", "snapshot", "backup"} {
		t.Run(kind, func(t *testing.T) {
			database, input, proof := accessorySettlementFixture(t, kind)
			consumer, err := NewAccessorySettlementConsumer(database, accessorySettlementEvidenceStub{proof: proof})
			if err != nil {
				t.Fatal(err)
			}
			var group sync.WaitGroup
			failures := make(chan error, 8)
			fresh := make(chan bool, 8)
			for range 8 {
				group.Add(1)
				go func() {
					defer group.Done()
					result, handled, consumeErr := consumer.ConsumeAddonSettlement(context.Background(), input)
					if consumeErr != nil {
						failures <- consumeErr
						return
					}
					if !handled || result == nil || result.FulfillmentCount != 0 {
						failures <- errors.New("addon was not consumed as zero new services")
						return
					}
					fresh <- !result.Reused
				}()
			}
			group.Wait()
			close(failures)
			close(fresh)
			for consumeErr := range failures {
				t.Error(consumeErr)
			}
			newCount := 0
			for first := range fresh {
				if first {
					newCount++
				}
			}
			if newCount != 1 {
				t.Fatalf("fresh deliveries=%d, want 1", newCount)
			}
			assertAccessorySettlementCounts(t, database, 1, 1, 1)
			var execution struct{ ServiceNo, ActionCode, OriginalRuntimeStatusCode, StatusCode string }
			if err = database.GetScan(context.Background(), &execution, `SELECT service_no,action_code,original_runtime_status_code,status_code FROM plugin_idc_core_server_operation_execution`); err != nil {
				t.Fatal(err)
			}
			if execution.ServiceNo != "srv_1" || execution.StatusCode != "ready" || (kind == "traffic" && execution.OriginalRuntimeStatusCode != "suspended") {
				t.Fatalf("wrong original task: %+v", execution)
			}
			for _, status := range []string{"result_unknown", "succeeded"} {
				if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_core_server_operation_execution SET status_code=?`, status); err != nil {
					t.Fatal(err)
				}
				if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_core_service_instance SET version=version+1`); err != nil {
					t.Fatal(err)
				}
				result, handled, consumeErr := consumer.ConsumeAddonSettlement(context.Background(), input)
				if consumeErr != nil || !handled || !result.Reused {
					t.Fatalf("terminal replay: %+v %v", result, consumeErr)
				}
				actual, getErr := database.GetValue(context.Background(), `SELECT status_code FROM plugin_idc_core_server_operation_execution`)
				if getErr != nil || actual.String() != status {
					t.Fatalf("replay reset existing task: %s %v", actual, getErr)
				}
			}
			forged := input
			forged.EventNo = "evt_other"
			if _, handled, consumeErr := consumer.ConsumeAddonSettlement(context.Background(), forged); consumeErr == nil || !handled {
				t.Fatal("different event identity accepted")
			}
			assertAccessorySettlementCounts(t, database, 1, 1, 1)
		})
	}
}

func TestAccessorySettlementRejectsBrokenEvidencePostgreSQL(t *testing.T) {
	for _, test := range []struct{ name, sql string }{
		{"unpaid order", `UPDATE plugin_idc_core_commerce_order SET status_code='pending_payment'`},
		{"unpaid invoice", `UPDATE plugin_idc_core_billing_invoice SET status_code='pending_payment'`},
		{"missing settlement", `DELETE FROM plugin_idc_core_financial_settlement`},
		{"missing intent", `DELETE FROM plugin_idc_core_business_operation`},
		{"customer changed", `UPDATE plugin_idc_core_service_instance SET customer_no='cus_other'`},
		{"invoice changed", `UPDATE plugin_idc_core_billing_invoice SET total_amount_minor=101`},
		{"currency changed", `UPDATE plugin_idc_core_server_accessory_quote SET currency_code='USD'`},
		{"service version changed", `UPDATE plugin_idc_core_service_instance SET version=3`},
		{"customer version changed", `UPDATE plugin_idc_core_customer_subject SET version=2`},
		{"parameters changed", `UPDATE plugin_idc_core_server_accessory_quote SET parameters='{"trafficGb":101}'::JSONB`},
		{"unsupported action", `UPDATE plugin_idc_core_server_accessory_quote SET action_code='start'`},
		{"purpose changed", `UPDATE plugin_idc_core_commerce_quote_snapshot SET purpose_code='purchase'`},
		{"purpose and invoice association changed", `UPDATE plugin_idc_core_commerce_quote_snapshot SET purpose_code='purchase'; UPDATE plugin_idc_core_business_operation SET invoice_no='inv_other'`},
		{"missing quote and invoice association", `DELETE FROM plugin_idc_core_commerce_quote_snapshot; UPDATE plugin_idc_core_business_operation SET invoice_no='inv_missing'`},
		{"price mismatch", `UPDATE plugin_idc_core_service_price_snapshot SET total_amount_minor=101`},
		{"payload changed", `UPDATE plugin_idc_core_outbox_event SET payload=jsonb_set(payload,'{totalAmountMinor}','101')`},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, input, proof := accessorySettlementFixture(t, "traffic")
			if _, err := database.Exec(context.Background(), test.sql); err != nil {
				t.Fatal(err)
			}
			consumer, err := NewAccessorySettlementConsumer(database, accessorySettlementEvidenceStub{proof: proof})
			if err != nil {
				t.Fatal(err)
			}
			if _, handled, consumeErr := consumer.ConsumeAddonSettlement(context.Background(), input); consumeErr == nil || !handled {
				t.Fatalf("broken addon fell through: handled=%v error=%v", handled, consumeErr)
			}
			assertAccessorySettlementCounts(t, database, 0, 0, 0)
		})
	}
}

// TestAccessorySettlementIndexedLookupPostgreSQL applies the exact production
// index statement twice and checks both queries on a selective 20,001-row fixture.
func TestAccessorySettlementIndexedLookupPostgreSQL(t *testing.T) {
	database, input, _ := accessorySettlementFixture(t, "traffic")
	ctx := context.Background()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve accessory index migration")
	}
	ddl, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "109-server-accessory-settlement-index.sql"))
	if err != nil {
		t.Fatal(err)
	}
	var indexDDL string
	for _, statement := range dialect.SplitSQLStatements(string(ddl)) {
		if strings.Contains(statement, "idx_plugin_idc_core_addon_order") {
			indexDDL = statement
			break
		}
	}
	if indexDDL == "" {
		t.Fatal("addon order index missing from production migration")
	}
	for range 2 {
		if _, err = database.Exec(ctx, indexDDL); err != nil {
			t.Fatalf("idempotent addon index migration: %+v", err)
		}
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation(operation_no,order_no)
SELECT 'aop_extra_'||number,'ord_extra_'||number FROM generate_series(1,20000) number;
ANALYZE plugin_idc_core_business_operation`); err != nil {
		t.Fatal(err)
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM pg_indexes
WHERE schemaname=current_schema() AND indexname='idx_plugin_idc_core_addon_order'`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("migration duplicated or missed index: count=%v err=%v", count, err)
	}
	payload, err := database.GetValue(ctx, `SELECT payload::TEXT FROM plugin_idc_core_outbox_event WHERE event_no=?`, input.EventNo)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, query string
		args        []any
	}{
		{"classification", accessorySettlementClassificationSQL, []any{"quo_1", input.OrderNo}},
		{"locked evidence", accessorySettlementEvidenceSQL, []any{input.OrderNo,
			input.SettlementNo, input.InvoiceNo, input.CustomerNo, input.CurrencyCode, input.TotalAmountMinor,
			input.EventNo, fulfillment.FinanciallySettledEventType, input.SettlementNo, payload.String(), `{"sourceCount":1}`}},
	} {
		plan, explainErr := database.GetValue(ctx, "EXPLAIN (FORMAT JSON) "+test.query, test.args...)
		if explainErr != nil {
			t.Fatalf("%s query plan: %+v", test.name, explainErr)
		}
		if !strings.Contains(plan.String(), "idx_plugin_idc_core_addon_order") {
			t.Fatalf("%s does not use bounded addon order lookup: %s", test.name, plan.String())
		}
		t.Logf("%s uses idx_plugin_idc_core_addon_order", test.name)
	}
}

func TestAccessorySettlementAuditFailureRollsBackPostgreSQL(t *testing.T) {
	database, input, proof := accessorySettlementFixture(t, "traffic")
	consumer, err := NewAccessorySettlementConsumer(database, accessorySettlementEvidenceStub{proof: proof})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `ALTER TABLE plugin_idc_core_business_operation_event ADD CONSTRAINT reject_event CHECK(FALSE)`); err != nil {
		t.Fatal(err)
	}
	if _, handled, consumeErr := consumer.ConsumeAddonSettlement(context.Background(), input); consumeErr == nil || !handled {
		t.Fatal("audit failure was hidden")
	}
	assertAccessorySettlementCounts(t, database, 0, 0, 0)
	state, getErr := database.GetValue(context.Background(), `SELECT status_code FROM plugin_idc_core_business_operation`)
	if getErr != nil || accessoryOperationStatus(state.String()) != accessoryAwaitingSettlement {
		t.Fatalf("failed transaction changed operation: %v %v", state, getErr)
	}
	if _, err = database.Exec(context.Background(), `ALTER TABLE plugin_idc_core_business_operation_event DROP CONSTRAINT reject_event`); err != nil {
		t.Fatal(err)
	}
	result, handled, err := consumer.ConsumeAddonSettlement(context.Background(), input)
	if err != nil || !handled || result.Reused {
		t.Fatalf("retry did not recover: %+v %v", result, err)
	}
	assertAccessorySettlementCounts(t, database, 1, 1, 1)
}

func assertAccessorySettlementCounts(t *testing.T, database gdb.DB, execution, event, receipt int) {
	t.Helper()
	for _, check := range []struct {
		table string
		count int
	}{
		{"plugin_idc_core_server_operation_execution", execution},
		{"plugin_idc_core_business_operation_event", event},
		{"plugin_idc_core_financial_settlement_delivery", receipt},
		{"plugin_idc_core_service_instance", 1},
	} {
		value, err := database.GetValue(context.Background(), `SELECT count(*) FROM `+check.table)
		if err != nil || value.Int() != check.count {
			t.Fatalf("%s count=%v want=%d error=%v", check.table, value, check.count, err)
		}
	}
}

func accessorySettlementFixture(t *testing.T, kind string) (gdb.DB, fulfillment.SettlementEvent, *billing.FinancialSettlement) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(transferPostgresDSNEnv))
	if dsn == "" {
		t.Skipf("set %s for isolated accessory settlement PostgreSQL tests", transferPostgresDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := transferTestSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_transfer_rule_") {
			t.Error("unexpected test schema")
			return
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, accessorySettlementFixtureSQL); err != nil {
		t.Fatal(err)
	}
	action, parameters, runtimeStatus := kind+"_create", `{"name": "test"}`, "stopped"
	if kind == "traffic" {
		action, parameters, runtimeStatus = "traffic_add", `{"trafficGb": 100}`, "suspended"
	}
	row := &accessorySettlementRow{OperationNo: "aop_1", QuoteNo: "quo_1", OrderNo: "ord_1", InvoiceNo: "inv_1", ServiceNo: "srv_1", ActionCode: action, Parameters: parameters, ExpectedServiceVersion: 2}
	content, digestErr := accessorySettlementIntentDigest(row)
	if digestErr != nil {
		t.Fatal(digestErr)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_quote SET action_code=?,accessory_kind=?,parameters=?::JSONB`, action, kind, parameters); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET content_digest=?`, content); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_service_instance SET runtime_status_code=?`, runtimeStatus); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 4, 3, 0, 0, 0, time.UTC)
	input := fulfillment.SettlementEvent{EventNo: "evt_1", SettlementNo: "fst_1", OrderNo: "ord_1", InvoiceNo: "inv_1", CustomerNo: "cus_1", CurrencyCode: "CNY", BasisCode: "funded_invoice", TotalAmountMinor: 100, SettledAtUnixMS: now.UnixMilli(), SourceCount: 1}
	payload, marshalErr := json.Marshal(map[string]any{"settlementNo": input.SettlementNo, "orderNo": input.OrderNo, "invoiceNo": input.InvoiceNo, "customerNo": input.CustomerNo, "currencyCode": input.CurrencyCode, "basisCode": input.BasisCode, "totalAmountMinor": input.TotalAmountMinor, "settledAtUnixMs": input.SettledAtUnixMS})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_outbox_event SET payload=?::JSONB`, string(payload)); err != nil {
		t.Fatal(err)
	}
	proof := &billing.FinancialSettlement{SettlementNo: input.SettlementNo, OrderNo: input.OrderNo, InvoiceNo: input.InvoiceNo, CustomerNo: input.CustomerNo, CurrencyCode: input.CurrencyCode, Basis: billing.SettlementBasisFundedInvoice, TotalAmountMinor: 100, PaidAmountMinor: 100, SettledAt: now, SourceSnapshotDigest: strings.Repeat("a", 64), IdentityEvidenceDigest: strings.Repeat("b", 64), Sources: []billing.FinancialSettlementSource{{AmountMinor: 100}}}
	return database, input, proof
}

const accessorySettlementFixtureSQL = `
CREATE TABLE plugin_idc_core_commerce_order(tenant_id INT DEFAULT 0,order_no TEXT PRIMARY KEY,quote_no TEXT DEFAULT 'quo_1',customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',status_code TEXT DEFAULT 'paid',item_count INT DEFAULT 1,requires_contract BOOLEAN DEFAULT FALSE,risk_decision_code TEXT DEFAULT 'approved',total_amount_minor BIGINT DEFAULT 100,subtotal_minor BIGINT DEFAULT 100,discount_minor BIGINT DEFAULT 0,net_amount_minor BIGINT DEFAULT 100,tax_amount_minor BIGINT DEFAULT 0);
CREATE TABLE plugin_idc_core_commerce_quote_snapshot(tenant_id INT DEFAULT 0,quote_no TEXT PRIMARY KEY,customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',purpose_code TEXT DEFAULT 'addon',customer_subject_version BIGINT DEFAULT 1,item_count INT DEFAULT 1,total_amount_minor BIGINT DEFAULT 100,subtotal_minor BIGINT DEFAULT 100,discount_minor BIGINT DEFAULT 0,net_amount_minor BIGINT DEFAULT 100,tax_amount_minor BIGINT DEFAULT 0);
CREATE TABLE plugin_idc_core_business_operation(tenant_id INT DEFAULT 0,operation_no TEXT PRIMARY KEY,order_no TEXT DEFAULT 'ord_1',invoice_no TEXT DEFAULT 'inv_1',quote_no TEXT DEFAULT 'quo_1',service_no TEXT DEFAULT 'srv_1',customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',operation_type_code TEXT DEFAULT 'addon',status_code TEXT DEFAULT 'awaiting_financial_settlement',settlement_no TEXT,financially_settled_at TIMESTAMPTZ,content_digest TEXT,version BIGINT DEFAULT 1,expected_service_version BIGINT DEFAULT 2,pool_no TEXT DEFAULT 'pool_1',resource_owner_code TEXT DEFAULT 'idc-resource',execution_mode_code TEXT DEFAULT 'unresolved',financial_settlement_required BOOLEAN DEFAULT TRUE,price_snapshot_no TEXT DEFAULT 'price_1',updated_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_server_accessory_quote(tenant_id INT DEFAULT 0,quote_no TEXT PRIMARY KEY,customer_no TEXT DEFAULT 'cus_1',service_no TEXT DEFAULT 'srv_1',currency_code TEXT DEFAULT 'CNY',action_code TEXT,accessory_kind TEXT,parameters JSONB,content_digest TEXT DEFAULT 'quote_digest',expected_service_version BIGINT DEFAULT 1,customer_subject_version BIGINT DEFAULT 1,total_amount_minor BIGINT DEFAULT 100,subtotal_minor BIGINT DEFAULT 100,net_amount_minor BIGINT DEFAULT 100,tax_amount_minor BIGINT DEFAULT 0);
CREATE TABLE plugin_idc_core_service_instance(tenant_id INT DEFAULT 0,service_no TEXT PRIMARY KEY,customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',version BIGINT DEFAULT 2,channel_no TEXT DEFAULT 'channel_1',pool_no TEXT DEFAULT 'pool_1',last_business_operation_no TEXT DEFAULT 'aop_1',entitlement_status_code TEXT DEFAULT 'active',runtime_status_code TEXT,provider_task_status_code TEXT DEFAULT 'none');
CREATE TABLE plugin_idc_core_customer_subject(tenant_id INT DEFAULT 0,customer_no TEXT PRIMARY KEY,version BIGINT DEFAULT 1,status_code TEXT DEFAULT 'active');
CREATE TABLE plugin_idc_core_service_traffic_suspension(tenant_id INT DEFAULT 0,service_no TEXT,active BOOLEAN DEFAULT TRUE);
CREATE TABLE plugin_idc_core_billing_invoice(tenant_id INT DEFAULT 0,invoice_no TEXT PRIMARY KEY,order_no TEXT DEFAULT 'ord_1',customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',status_code TEXT DEFAULT 'paid',item_count INT DEFAULT 1,source_snapshot_digest TEXT DEFAULT 'quote_digest',total_amount_minor BIGINT DEFAULT 100,subtotal_minor BIGINT DEFAULT 100,discount_minor BIGINT DEFAULT 0,net_amount_minor BIGINT DEFAULT 100,tax_amount_minor BIGINT DEFAULT 0);
CREATE TABLE plugin_idc_core_service_price_snapshot(tenant_id INT DEFAULT 0,price_snapshot_no TEXT PRIMARY KEY,source_quote_no TEXT DEFAULT 'quo_1',source_quote_item_no TEXT DEFAULT 'qi_1',service_no TEXT DEFAULT 'srv_1',customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',phase_code TEXT DEFAULT 'addon',product_no TEXT DEFAULT 'prod_1',version_no TEXT DEFAULT 'ver_1',total_amount_minor BIGINT DEFAULT 100,subtotal_minor BIGINT DEFAULT 100,discount_minor BIGINT DEFAULT 0,net_amount_minor BIGINT DEFAULT 100,tax_amount_minor BIGINT DEFAULT 0);
CREATE TABLE plugin_idc_core_commerce_order_item(tenant_id INT DEFAULT 0,order_item_no TEXT PRIMARY KEY,order_no TEXT DEFAULT 'ord_1',quote_no TEXT DEFAULT 'quo_1',quote_item_no TEXT DEFAULT 'qi_1',currency_code TEXT DEFAULT 'CNY',product_no TEXT DEFAULT 'prod_1',version_no TEXT DEFAULT 'ver_1',total_amount_minor BIGINT DEFAULT 100);
CREATE TABLE plugin_idc_core_billing_invoice_item(tenant_id INT DEFAULT 0,invoice_no TEXT DEFAULT 'inv_1',order_no TEXT DEFAULT 'ord_1',order_item_no TEXT DEFAULT 'oi_1',customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',total_amount_minor BIGINT DEFAULT 100);
CREATE TABLE plugin_idc_core_financial_settlement(tenant_id INT DEFAULT 0,settlement_no TEXT PRIMARY KEY,order_no TEXT DEFAULT 'ord_1',invoice_no TEXT DEFAULT 'inv_1',customer_no TEXT DEFAULT 'cus_1',currency_code TEXT DEFAULT 'CNY',total_amount_minor BIGINT DEFAULT 100);
CREATE TABLE plugin_idc_core_outbox_event(tenant_id INT DEFAULT 0,event_no TEXT PRIMARY KEY,event_type TEXT DEFAULT 'commerce.order.financially_settled',event_version INT DEFAULT 1,topic TEXT DEFAULT 'commerce.order.financially_settled',aggregate_type TEXT DEFAULT 'commerce_order',aggregate_no TEXT DEFAULT 'ord_1',correlation_no TEXT DEFAULT 'fst_1',payload JSONB,metadata JSONB DEFAULT '{"sourceCount":1}');
CREATE TABLE plugin_idc_core_server_operation_execution(tenant_id INT,execution_no TEXT PRIMARY KEY,operation_no TEXT UNIQUE,service_no TEXT,action_code TEXT,resource_owner_code TEXT,channel_no TEXT,pool_no TEXT,expected_service_version BIGINT,original_runtime_status_code TEXT,parameters JSONB,secret_bindings JSONB,status_code TEXT,request_digest TEXT,version BIGINT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
CREATE UNIQUE INDEX active_service ON plugin_idc_core_server_operation_execution(service_no) WHERE status_code IN('ready','pending','running','result_unknown');
CREATE TABLE plugin_idc_core_business_operation_event(tenant_id INT,event_no TEXT PRIMARY KEY,operation_no TEXT,service_no TEXT,event_sequence INT,from_status_code TEXT,to_status_code TEXT,reason_code TEXT,actor_type_code TEXT,actor_reference TEXT,evidence_digest TEXT,content_digest TEXT,created_at TIMESTAMPTZ,UNIQUE(operation_no,event_sequence));
CREATE TABLE plugin_idc_core_financial_settlement_delivery(tenant_id INT,delivery_no TEXT PRIMARY KEY,event_no TEXT UNIQUE,settlement_no TEXT UNIQUE,event_type TEXT,order_no TEXT,invoice_no TEXT,customer_no TEXT,currency_code TEXT,fulfillment_count INT,ready_count INT,waiting_gate_count INT,manual_review_count INT,evidence_digest TEXT,content_digest TEXT,created_at TIMESTAMPTZ);
ALTER TABLE plugin_idc_core_business_operation ADD COLUMN subject_type_code TEXT DEFAULT 'service', ADD COLUMN subject_no TEXT, ADD COLUMN expected_subject_version BIGINT, ADD COLUMN channel_no TEXT, ADD COLUMN execution_started_at TIMESTAMPTZ, ADD COLUMN completed_at TIMESTAMPTZ;
ALTER TABLE plugin_idc_core_server_accessory_quote ADD COLUMN subject_type_code TEXT DEFAULT 'service', ADD COLUMN subject_no TEXT, ADD COLUMN expected_subject_version BIGINT, ADD COLUMN native_target_snapshot JSONB DEFAULT '{}'::JSONB;
ALTER TABLE plugin_idc_core_service_price_snapshot ADD COLUMN subject_type_code TEXT DEFAULT 'service', ADD COLUMN subject_no TEXT;
ALTER TABLE plugin_idc_core_business_operation_event ADD COLUMN subject_type_code TEXT DEFAULT 'service', ADD COLUMN subject_no TEXT;
CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT DEFAULT 0,entitlement_no TEXT PRIMARY KEY,customer_no TEXT,currency_code TEXT,version BIGINT,status_code TEXT,entitlement_owner_id TEXT,upstream_service_no TEXT);
INSERT INTO plugin_idc_core_commerce_order(order_no) VALUES('ord_1');
INSERT INTO plugin_idc_core_commerce_quote_snapshot(quote_no) VALUES('quo_1');
INSERT INTO plugin_idc_core_business_operation(operation_no) VALUES('aop_1');
INSERT INTO plugin_idc_core_server_accessory_quote(quote_no) VALUES('quo_1');
INSERT INTO plugin_idc_core_service_instance(service_no) VALUES('srv_1');
INSERT INTO plugin_idc_core_customer_subject(customer_no) VALUES('cus_1');
INSERT INTO plugin_idc_core_service_traffic_suspension(service_no) VALUES('srv_1');
INSERT INTO plugin_idc_core_billing_invoice(invoice_no) VALUES('inv_1');
INSERT INTO plugin_idc_core_service_price_snapshot(price_snapshot_no) VALUES('price_1');
INSERT INTO plugin_idc_core_commerce_order_item(order_item_no) VALUES('oi_1');
INSERT INTO plugin_idc_core_billing_invoice_item(invoice_no) VALUES('inv_1');
INSERT INTO plugin_idc_core_financial_settlement(settlement_no) VALUES('fst_1');
INSERT INTO plugin_idc_core_outbox_event(event_no) VALUES('evt_1');`
