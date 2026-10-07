// This file verifies deterministic multi-campaign allocation of authoritative marketing amounts.

package marketing

import (
	"context"
	"strings"
	"testing"
)

func TestEffectReconcilerPostgreSQLAllocatesRemaindersDeterministically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	currentSchema, err := database.GetValue(ctx, `SELECT current_schema()`)
	if err != nil || !strings.HasPrefix(currentSchema.String(), "linapro_promotion_test_") {
		t.Fatalf("refusing to replace tables outside isolated marketing schema: schema=%q err=%v", currentSchema.String(), err)
	}
	if _, err = database.Exec(ctx, `
DROP TABLE plugin_idc_core_marketing_effect_fact CASCADE;
DROP TABLE plugin_idc_core_billing_refund_allocation CASCADE;
DROP TABLE plugin_idc_core_billing_payment_allocation CASCADE;
DROP TABLE plugin_idc_core_billing_invoice CASCADE;
DROP TABLE plugin_idc_core_commerce_order_cost_snapshot CASCADE;
DROP TABLE plugin_idc_core_marketing_campaign_reservation CASCADE;
DROP TABLE plugin_idc_core_commerce_order CASCADE;

CREATE TABLE plugin_idc_core_commerce_order (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 order_no TEXT NOT NULL UNIQUE, quote_no TEXT NOT NULL UNIQUE,
 customer_no TEXT NOT NULL, currency_code TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE plugin_idc_core_marketing_campaign_reservation (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 reservation_no TEXT NOT NULL UNIQUE, campaign_no TEXT NOT NULL,
 quote_reference TEXT NOT NULL, status_code TEXT NOT NULL,
 quoted_value_minor BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_billing_invoice (
 invoice_no TEXT PRIMARY KEY, order_no TEXT, customer_no TEXT NOT NULL,
 currency_code TEXT NOT NULL, settled_at TIMESTAMPTZ
);
CREATE TABLE plugin_idc_core_billing_payment_allocation (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 allocation_no TEXT NOT NULL UNIQUE, invoice_no TEXT NOT NULL,
 amount_minor BIGINT NOT NULL, direction_code TEXT NOT NULL,
 content_digest TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE plugin_idc_core_billing_refund_allocation (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 refund_allocation_no TEXT NOT NULL UNIQUE, invoice_no TEXT NOT NULL,
 amount_minor BIGINT NOT NULL, direction_code TEXT NOT NULL,
 content_digest TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE plugin_idc_core_commerce_order_cost_snapshot (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 order_no TEXT NOT NULL, status_code TEXT NOT NULL, total_cost_minor BIGINT
);
CREATE TABLE plugin_idc_core_marketing_effect_fact (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL, fact_no TEXT NOT NULL UNIQUE, campaign_no TEXT NOT NULL,
 customer_no TEXT, event_type_code TEXT NOT NULL, currency_code TEXT,
 amount_minor BIGINT NOT NULL, source_type_code TEXT NOT NULL,
 source_reference TEXT NOT NULL, content_digest TEXT NOT NULL,
 occurred_at TIMESTAMPTZ NOT NULL,
 UNIQUE (source_type_code, source_reference, event_type_code, campaign_no)
);

INSERT INTO plugin_idc_core_commerce_order
 (order_no,quote_no,customer_no,currency_code) VALUES
 ('order-payment','quote-payment','customer-a','CNY'),
 ('order-refund','quote-refund','customer-a','CNY'),
 ('order-cost','quote-cost','customer-a','CNY');
INSERT INTO plugin_idc_core_marketing_campaign_reservation
 (reservation_no,campaign_no,quote_reference,status_code,quoted_value_minor) VALUES
 ('res-payment-a','campaign-a','quote-payment','consumed',60),
 ('res-payment-b','campaign-b','quote-payment','consumed',40),
 ('res-refund-b','campaign-b','quote-refund','consumed',50),
 ('res-refund-a','campaign-a','quote-refund','consumed',50),
 ('res-cost-a','campaign-a','quote-cost','consumed',60),
 ('res-cost-b','campaign-b','quote-cost','consumed',40);
INSERT INTO plugin_idc_core_billing_invoice
 (invoice_no,order_no,customer_no,currency_code,settled_at) VALUES
 ('invoice-payment','order-payment','customer-a','CNY',CURRENT_TIMESTAMP),
 ('invoice-refund','order-refund','customer-a','CNY',CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_payment_allocation
 (allocation_no,invoice_no,amount_minor,direction_code,content_digest)
 VALUES ('allocation-payment','invoice-payment',101,'apply',repeat('a',64));
INSERT INTO plugin_idc_core_billing_refund_allocation
 (refund_allocation_no,invoice_no,amount_minor,direction_code,content_digest)
 VALUES ('allocation-refund','invoice-refund',5,'apply',repeat('b',64));
INSERT INTO plugin_idc_core_commerce_order_cost_snapshot
 (order_no,status_code,total_cost_minor)
 VALUES ('order-cost','frozen',7);`); err != nil {
		t.Fatalf("seed deterministic marketing allocation fixture: %v", err)
	}
	for name, query := range map[string]string{
		"payment": reconcilePaymentFactsSQL,
		"refund":  reconcileRefundFactsSQL,
		"cost":    reconcileCostFactsSQL,
	} {
		if _, err = database.Exec(ctx, query, 100); err != nil {
			t.Fatalf("reconcile %s facts: %v", name, err)
		}
	}
	type allocatedFact struct {
		SourceType  string `orm:"source_type_code"`
		CampaignNo  string `orm:"campaign_no"`
		AmountMinor int64  `orm:"amount_minor"`
	}
	var facts []allocatedFact
	if err = database.GetScan(ctx, &facts, `SELECT source_type_code,campaign_no,amount_minor
FROM plugin_idc_core_marketing_effect_fact
ORDER BY source_type_code,campaign_no`); err != nil {
		t.Fatalf("read allocated marketing facts: %v", err)
	}
	want := []allocatedFact{
		{SourceType: "order_cost", CampaignNo: "campaign-a", AmountMinor: 5},
		{SourceType: "order_cost", CampaignNo: "campaign-b", AmountMinor: 2},
		{SourceType: "payment_allocation", CampaignNo: "campaign-a", AmountMinor: 61},
		{SourceType: "payment_allocation", CampaignNo: "campaign-b", AmountMinor: 40},
		{SourceType: "refund_allocation", CampaignNo: "campaign-a", AmountMinor: 3},
		{SourceType: "refund_allocation", CampaignNo: "campaign-b", AmountMinor: 2},
	}
	if len(facts) != len(want) {
		t.Fatalf("allocated fact count=%d want=%d facts=%+v", len(facts), len(want), facts)
	}
	for index := range want {
		if facts[index] != want[index] {
			t.Fatalf("allocated fact[%d]=%+v want=%+v", index, facts[index], want[index])
		}
	}
	for name, query := range map[string]string{
		"payment replay": reconcilePaymentFactsSQL,
		"refund replay":  reconcileRefundFactsSQL,
		"cost replay":    reconcileCostFactsSQL,
	} {
		result, replayErr := database.Exec(ctx, query, 100)
		if replayErr != nil {
			t.Fatalf("%s: %v", name, replayErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 0 {
			t.Fatalf("%s rows=%d err=%v", name, rows, rowsErr)
		}
	}
}
