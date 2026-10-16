// This file verifies accrual, maturity and incremental refunds against all real
// PostgreSQL commission constraints and account projection triggers.
package commission

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

func TestCommissionRenewalRuntimePostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	seedCommissionRenewalFixture(t, ctx, database)
	rules, err := saleschannel.NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	consumerValue, err := NewConsumer(database, rules)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := consumerValue.AccrueRenewals(ctx, 10); failures <- err }()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertCommissionAmount(t, ctx, database, `SELECT SUM(original_amount_minor) FROM plugin_idc_core_commission_accrual`, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_order_receipt`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT SUM(received_amount_minor) FROM plugin_idc_core_sales_payment_fact`, 10000)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_owner`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT pending_amount_minor FROM plugin_idc_core_sales_commission_account`, 1000)
	// First partial refund happens before maturity. The next one must use cumulative actual refunds.
	consumeCommissionRefundFixture(t, ctx, database, consumerValue, "refund_1", 1500, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT pending_amount_minor FROM plugin_idc_core_sales_commission_account`, 750)
	if _, err = rules.MatureDueCommissions(ctx, time.Now().Add(10*365*24*time.Hour), 100); err != nil {
		t.Fatal(err)
	}
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 750)
	consumeCommissionRefundFixture(t, ctx, database, consumerValue, "refund_2", 1500, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 500)
	assertCommissionAmount(t, ctx, database, `SELECT reversed_amount_minor FROM plugin_idc_core_sales_commission_account`, 500)
	consumeCommissionRefundFixture(t, ctx, database, consumerValue, "refund_3", 3000, 2000)
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT reversed_amount_minor FROM plugin_idc_core_sales_commission_account`, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_accrual WHERE status_code='reversed'`, 2)
}

func TestCommissionPurchaseAndHistoricalRulesPostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Exec(`SET LOCAL session_replication_role=replica;
 INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name) VALUES('purchase_customer','individual','active','Purchase customer');
 INSERT INTO plugin_idc_core_sales_group(group_no,group_code,name_zh_cn,name_en_us) VALUES('sg_purchase','purchase','销售','Sales');
 INSERT INTO plugin_idc_core_sales_group_member(membership_no,group_no,staff_user_id,effective_at) VALUES('sm_purchase','sg_purchase',201,CURRENT_TIMESTAMP-INTERVAL '2 day');
 INSERT INTO plugin_idc_core_sales_customer_assignment(assignment_no,customer_no,membership_no,source_code,effective_at) VALUES('sa_purchase','purchase_customer','sm_purchase','manual',CURRENT_TIMESTAMP-INTERVAL '2 day');
 INSERT INTO plugin_idc_core_sales_commission_rule(rule_no,beneficiary_type_code,operation_type_code,currency_code,calculation_type_code,rate_basis_points,maturity_days,status_code,effective_at,created_at,version,rule_digest)
 VALUES('scr_purchase','sales','new_purchase','CNY','rate',1000,7,'published',CURRENT_TIMESTAMP-INTERVAL '2 day',CURRENT_TIMESTAMP-INTERVAL '2 day',1,repeat('1',64)),
 ('scr_purchase_retired','sales','new_purchase','CNY','rate',5000,7,'retired',CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,2,repeat('2',64));
 INSERT INTO plugin_idc_core_commerce_order(order_no,customer_no,quote_no,currency_code,source_code,status_code,risk_decision_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,checkout_idempotency_digest,checkout_request_digest,payment_due_at,paid_at)
 VALUES('ord_purchase','purchase_customer','quote_purchase','CNY','portal','paid','approved',1,10000,10000,10000,repeat('1',64),repeat('2',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '1 minute'),
 ('ord_zero','purchase_customer','quote_zero','CNY','portal','paid','approved',1,0,0,0,repeat('3',64),repeat('4',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '1 minute');
 INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,delivery_no,source_intent_no,customer_no,order_no,order_item_no,product_no,version_no,unit_sequence,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,started_at)
 VALUES('ent_purchase','del_purchase','intent_purchase','purchase_customer','ord_purchase','original_item','product_purchase','version_purchase',1,'interface_purchase','idc-server','1.0.0','1.0.0',repeat('1',64),CURRENT_TIMESTAMP-INTERVAL '1 day');
 INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,order_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,paid_amount_minor,refundable_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at)
 VALUES('inv_purchase','purchase_customer','ord_purchase','order','CNY','paid',1,10000,10000,10000,10000,10000,repeat('1',64),repeat('2',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '1 hour',CURRENT_TIMESTAMP-INTERVAL '1 minute'),
 ('inv_zero','purchase_customer','ord_zero','order','CNY','paid',1,0,0,0,0,0,repeat('3',64),repeat('4',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '1 hour',CURRENT_TIMESTAMP-INTERVAL '1 minute');
 INSERT INTO plugin_idc_core_billing_invoice_item(invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,source_reference,description_zh_cn,description_en_us,subtotal_minor,net_amount_minor,total_amount_minor,refundable_cap_minor,content_digest)
 VALUES('bit_purchase','inv_purchase','purchase_customer','CNY',1,'renewal','ent_purchase','商品','Product',10000,10000,10000,10000,repeat('1',64));
 INSERT INTO plugin_idc_core_financial_settlement(settlement_no,order_no,invoice_no,customer_no,currency_code,basis_code,total_amount_minor,paid_amount_minor,source_count,identity_evidence_digest,source_snapshot_digest,content_digest,settled_at)
 VALUES('fst_purchase','ord_purchase','inv_purchase','purchase_customer','CNY','funded_invoice',10000,10000,1,repeat('1',64),repeat('2',64),repeat('3',64),CURRENT_TIMESTAMP-INTERVAL '1 minute'),
 ('fst_zero','ord_zero','inv_zero','purchase_customer','CNY','zero_amount',0,0,1,repeat('4',64),repeat('5',64),repeat('6',64),CURRENT_TIMESTAMP-INTERVAL '1 minute');`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	rules, err := saleschannel.NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	consumerValue, err := NewConsumer(database, rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"purchase", "zero"} {
		payload, err := database.GetValue(ctx, `SELECT json_build_object('settlementNo',settlement_no,'orderNo',order_no,'invoiceNo',invoice_no,'customerNo',customer_no,'currencyCode',currency_code,'basisCode',basis_code,'totalAmountMinor',total_amount_minor,'settledAtUnixMs',FLOOR(EXTRACT(EPOCH FROM settled_at)*1000)::bigint)::text FROM plugin_idc_core_financial_settlement WHERE settlement_no=?`, "fst_"+suffix)
		if err != nil {
			t.Fatal(err)
		}
		event := outbox.ClaimedEvent{Event: outbox.Event{EventNo: "event_" + suffix, EventType: AccrualEventType, EventVersion: 1, Topic: AccrualEventType, AggregateType: "commerce_order", AggregateNo: "ord_" + suffix, Payload: json.RawMessage(payload.String())}}
		forged := event
		var tampered map[string]any
		if err = json.Unmarshal(event.Payload, &tampered); err != nil {
			t.Fatal(err)
		}
		tampered["customerNo"] = "other_customer"
		forged.Payload, err = json.Marshal(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err = consumerValue.Consume(ctx, forged); err == nil {
			t.Fatal("forged settlement customer accepted")
		}
		if err = consumerValue.Consume(ctx, event); err != nil {
			t.Fatal(err)
		}
		if err = consumerValue.Consume(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	assertCommissionAmount(t, ctx, database, `SELECT SUM(original_amount_minor) FROM plugin_idc_core_commission_accrual`, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact WHERE operation_type_code='new_purchase'`, 2)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact WHERE order_no='ord_zero' AND received_amount_minor=0`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_order_receipt`, 2)
	if _, err = rules.MatureDueCommissions(ctx, time.Now().Add(30*24*time.Hour), 100); err != nil {
		t.Fatal(err)
	}
	assertCommissionAmount(t, ctx, database, `SELECT pending_amount_minor FROM plugin_idc_core_sales_commission_account`, 1000)
	// A future caller clock must never mature the seven-day commission early.
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 0)
}

func consumeCommissionRefundFixture(t *testing.T, ctx context.Context, database gdb.DB, consumerValue Consumer, refund string, first, second int64) {
	t.Helper()
	consumeCommissionRefundForInvoice(t, ctx, database, consumerValue, refund, "inv_commission", "bit_commission_1", "bit_commission_2", first, second)
}
func consumeCommissionRefundForInvoice(t *testing.T, ctx context.Context, database gdb.DB, consumerValue Consumer, refund, invoiceNo, itemFirst, itemSecond string, first, second int64) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`SET LOCAL session_replication_role=replica`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO plugin_idc_core_refund_execution_command(command_no,refund_no,refund_source_no,status_code,destination_code,currency_code,amount_minor,source_reference,request_digest,result_code,completed_at) VALUES(?,?,?,'succeeded','same_currency_balance','CNY',?,?,repeat('4',64),'succeeded',CURRENT_TIMESTAMP)`, "cmd_"+refund, refund, "source_"+refund, first+second, "payment_"+refund); err != nil {
			return err
		}
		for i, amount := range []int64{first, second} {
			if _, err := tx.Exec(`INSERT INTO plugin_idc_core_billing_refund_allocation(refund_allocation_no,invoice_no,invoice_item_no,customer_no,currency_code,payment_allocation_no,refund_source_type_code,refund_source_reference,ledger_transaction_no,ledger_transaction_type_code,amount_minor,content_digest) VALUES(?,?,?,'commission_customer','CNY','bpa_commission','balance_credit',?,?,'refund',?,repeat('5',64))`, fmt.Sprintf("bra_%s_%d", refund, i), invoiceNo, []string{itemFirst, itemSecond}[i], "cmd_"+refund, "ledger_"+refund, amount); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(refundPayload{RefundNo: refund, InvoiceNo: invoiceNo, CustomerNo: "commission_customer", CurrencyCode: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	event := outbox.ClaimedEvent{Event: outbox.Event{EventNo: "event_" + refund, EventType: RefundReversalEventType, EventVersion: 1, Topic: RefundReversalEventType, AggregateType: "refund_request", AggregateNo: refund, Payload: payload}}
	if err = consumerValue.Consume(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err = consumerValue.Consume(ctx, event); err != nil {
		t.Fatalf("replay: %v", err)
	}
}

func assertCommissionAmount(t *testing.T, ctx context.Context, database gdb.DB, query string, want int64) {
	t.Helper()
	value, err := database.GetValue(ctx, query)
	if err != nil || value.Int64() != want {
		t.Fatalf("%s: got=%v want=%d err=%v", query, value, want, err)
	}
}

func seedCommissionRenewalFixture(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Exec(`SET LOCAL session_replication_role=replica;
 INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name) VALUES('commission_customer','individual','active','Commission customer');
 INSERT INTO plugin_idc_core_sales_group(group_no,group_code,name_zh_cn,name_en_us) VALUES('sg_commission','commission','销售','Sales');
 INSERT INTO plugin_idc_core_sales_group_member(membership_no,group_no,staff_user_id,effective_at) VALUES('sm_commission','sg_commission',101,CURRENT_TIMESTAMP-INTERVAL '2 day');
 INSERT INTO plugin_idc_core_sales_customer_assignment(assignment_no,customer_no,membership_no,source_code,effective_at) VALUES('sa_commission','commission_customer','sm_commission','manual',CURRENT_TIMESTAMP-INTERVAL '2 day');
 INSERT INTO plugin_idc_core_sales_commission_rule(rule_no,beneficiary_type_code,operation_type_code,currency_code,calculation_type_code,fixed_amount_minor,maturity_days,status_code,effective_at,created_at,version,rule_digest)
 VALUES('scr_commission','sales','renewal','CNY','fixed',1000,0,'published',CURRENT_TIMESTAMP-INTERVAL '2 day',CURRENT_TIMESTAMP-INTERVAL '2 day',1,repeat('1',64));
 INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,delivery_no,source_intent_no,customer_no,order_no,order_item_no,product_no,version_no,unit_sequence,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,started_at)
 VALUES('ent_commission','del_commission','intent_commission','commission_customer','original_order','original_item','product_commission','version_commission',1,'interface_commission','idc-server','1.0.0','1.0.0',repeat('1',64),CURRENT_TIMESTAMP-INTERVAL '1 day');
 INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,paid_amount_minor,refundable_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at)
 VALUES('inv_commission','commission_customer','renewal','CNY','paid',2,10000,10000,10000,10000,10000,repeat('1',64),repeat('2',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '1 hour',CURRENT_TIMESTAMP-INTERVAL '1 minute');
 INSERT INTO plugin_idc_core_billing_invoice_item(invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,source_reference,description_zh_cn,description_en_us,subtotal_minor,net_amount_minor,total_amount_minor,refundable_cap_minor,content_digest)
 VALUES('bit_commission_1','inv_commission','commission_customer','CNY',1,'renewal','ent_commission','续费','Renewal',6000,6000,6000,6000,repeat('1',64)),
 ('bit_commission_2','inv_commission','commission_customer','CNY',2,'renewal','ent_commission','续费','Renewal',4000,4000,4000,4000,repeat('2',64));
 INSERT INTO plugin_idc_core_billing_payment_allocation(allocation_no,invoice_no,customer_no,currency_code,payment_source_type_code,payment_source_reference,ledger_transaction_no,ledger_transaction_type_code,amount_minor,content_digest)
 VALUES('bpa_commission','inv_commission','commission_customer','CNY','balance','paid_commission','ledger_commission','balance_payment',10000,repeat('3',64));`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommissionPaidRefundRecoveryPostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	seedCommissionRenewalFixture(t, ctx, database)
	rules, err := saleschannel.NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	consumerValue, err := NewConsumer(database, rules)
	if err != nil {
		t.Fatal(err)
	}
	runtime := consumerValue.(*consumer)
	if _, err = runtime.AccrueRenewals(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.Mature(ctx, 10); err != nil {
		t.Fatal(err)
	}
	// The payout fixture uses the real account ledger trigger and append-only payout allocation.
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		account, err := runtime.lockAccount(ctx, beneficiary{Kind: beneficiarySales, Reference: "101"}, "CNY", false)
		if err != nil {
			return err
		}
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_offline_payout(payout_no,account_no,currency_code,amount_minor,evidence_reference,paid_at,recorded_by,request_digest) VALUES('cop_test',?,'CNY',1000,'receipt:test:001',CURRENT_TIMESTAMP,1,repeat('7',64))`, account.AccountNo); err != nil {
			return err
		}
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_offline_payout_item(payout_no,commission_no,amount_minor) SELECT 'cop_test',commission_no,matured_amount_minor FROM plugin_idc_core_commission_accrual`); err != nil {
			return err
		}
		return runtime.appendAccountEntry(ctx, account, "payout", "commission_offline_payout", "cop_test", 0, 0, -1000, 1000, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	customerService := rules.(saleschannel.CustomerService)
	customerOverview, err := customerService.LoadCustomerCommissionOverview(ctx, "101", "CNY", 20)
	if err != nil || len(customerOverview.Accounts) != 0 || len(customerOverview.Entries) != 0 {
		t.Fatalf("employee commission leaked to matching customer number: %+v %v", customerOverview, err)
	}
	customerHistory, err := customerService.ListCustomerCommissionSettlements(ctx, "101", "CNY", 20)
	if err != nil || len(customerHistory.Items) != 0 {
		t.Fatalf("employee payout leaked to matching customer number: %+v %v", customerHistory, err)
	}
	consumeCommissionRefundFixture(t, ctx, database, runtime, "paid_refund", 3000, 2000)
	assertCommissionAmount(t, ctx, database, `SELECT paid_amount_minor FROM plugin_idc_core_sales_commission_account`, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_recovery`, 500)
	seedAdditionalCommissionInvoice(t, ctx, database, "next")
	if _, err = runtime.AccrueRenewals(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.Mature(ctx, 10); err != nil {
		t.Fatal(err)
	}
	assertCommissionAmount(t, ctx, database, `SELECT paid_amount_minor FROM plugin_idc_core_sales_commission_account`, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 500)
	assertCommissionAmount(t, ctx, database, `SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_recovery_offset`, 500)
	assertCommissionAmount(t, ctx, database, `SELECT (SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_recovery)-(SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_recovery_offset)`, 0)
	// Refunding the commission that repaid old debt must restore the consumed debt.
	consumeCommissionRefundForInvoice(t, ctx, database, runtime, "offset_refund", "inv_next", "bit_next_1", "bit_next_2", 6000, 4000)
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT (SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_recovery)-(SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_recovery_offset)`, 500)
	assertCommissionAmount(t, ctx, database, `SELECT credited_amount_minor-pending_amount_minor-available_amount_minor-frozen_amount_minor-paid_amount_minor-reversed_amount_minor FROM plugin_idc_core_sales_commission_account`, 0)
}

func seedAdditionalCommissionInvoice(t *testing.T, ctx context.Context, database gdb.DB, suffix string) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`SET LOCAL session_replication_role=replica`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,paid_amount_minor,refundable_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at) VALUES(?,'commission_customer','renewal','CNY','paid',2,10000,10000,10000,10000,10000,repeat('8',64),repeat('9',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, "inv_"+suffix); err != nil {
			return err
		}
		for i, amount := range []int64{6000, 4000} {
			if _, err := tx.Exec(`INSERT INTO plugin_idc_core_billing_invoice_item(invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,source_reference,description_zh_cn,description_en_us,subtotal_minor,net_amount_minor,total_amount_minor,refundable_cap_minor,content_digest) VALUES(?,?,'commission_customer','CNY',?,'renewal','ent_commission','续费','Renewal',?,?,?,?,repeat('8',64))`, fmt.Sprintf("bit_%s_%d", suffix, i+1), "inv_"+suffix, i+1, amount, amount, amount, amount); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO plugin_idc_core_billing_payment_allocation(allocation_no,invoice_no,customer_no,currency_code,payment_source_type_code,payment_source_reference,ledger_transaction_no,ledger_transaction_type_code,amount_minor,content_digest) VALUES(?,?,'commission_customer','CNY','balance',?,?,'balance_payment',10000,repeat('9',64))`, "bpa_"+suffix, "inv_"+suffix, "paid_"+suffix, "ledger_"+suffix)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommissionNoRuleReceiptPostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	seedCommissionRenewalFixture(t, ctx, database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_rule(rule_no,beneficiary_type_code,operation_type_code,currency_code,calculation_type_code,fixed_amount_minor,maturity_days,status_code,effective_at,version,rule_digest) VALUES('scr_retired','sales','renewal','CNY','fixed',1000,0,'retired',CURRENT_TIMESTAMP-INTERVAL '1 hour',2,repeat('4',64))`); err != nil {
		t.Fatal(err)
	}
	rules, err := saleschannel.NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	consumerValue, err := NewConsumer(database, rules)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = consumerValue.AccrueRenewals(ctx, 10); err != nil {
		t.Fatal(err)
	}
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_accrual`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_order_receipt`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact WHERE received_amount_minor=10000`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_owner WHERE beneficiary_type_code='sales' AND beneficiary_reference='101'`, 1)
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_rule(rule_no,beneficiary_type_code,operation_type_code,currency_code,calculation_type_code,fixed_amount_minor,maturity_days,status_code,effective_at,version,rule_digest) VALUES('scr_later','sales','renewal','CNY','fixed',1000,0,'published',CURRENT_TIMESTAMP-INTERVAL '1 hour',3,repeat('5',64))`); err != nil {
		t.Fatal(err)
	}
	if _, err = consumerValue.AccrueRenewals(ctx, 10); err != nil {
		t.Fatal(err)
	}
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_accrual`, 0)
	consumeCommissionRefundFixture(t, ctx, database, consumerValue, "refund_without_commission", 1500, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT SUM(received_amount_minor) FROM plugin_idc_core_sales_payment_fact`, 10000)
	assertCommissionAmount(t, ctx, database, `SELECT SUM(r.amount_minor) FROM plugin_idc_core_billing_refund_allocation r JOIN plugin_idc_core_sales_payment_owner o USING(invoice_no) WHERE o.beneficiary_type_code='sales' AND o.beneficiary_reference='101'`, 2500)
}
