// This file verifies owner-side channel reconciliation counts and numeric pages.
package backend

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func TestChannelFinancePageValidation(t *testing.T) {
	for _, page := range []channelcommercecap.FinancePage{{Page: -1}, {Page: 1000001}, {PageSize: -1}, {PageSize: 101}} {
		_, err := normalizeChannelFinancePages([]channelcommercecap.FinancePages{{Invoices: page}})
		if err == nil {
			t.Fatalf("invalid finance page accepted: %+v", page)
		}
	}
}

func TestChannelFinancePagesPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skip("PostgreSQL pagination DSN is not configured")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newCatalogIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); cleanupErr != nil {
			t.Error(cleanupErr)
		}
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_core_billing_invoice(id BIGINT,tenant_id INT,invoice_no TEXT,order_no TEXT,currency_code TEXT,status_code TEXT,total_amount_minor BIGINT DEFAULT 100,paid_amount_minor BIGINT DEFAULT 100,credit_amount_minor BIGINT DEFAULT 0,refunded_amount_minor BIGINT DEFAULT 0,created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE plugin_idc_core_channel_order_attribution(tenant_id INT,channel_id TEXT,payer_customer_no TEXT,order_no TEXT)`,
		`CREATE TABLE plugin_idc_core_credit_statement(id BIGINT,tenant_id INT,customer_no TEXT,statement_no TEXT,currency_code TEXT,status_code TEXT,total_amount_minor BIGINT DEFAULT 100,paid_amount_minor BIGINT DEFAULT 0,outstanding_amount_minor BIGINT DEFAULT 100,issued_at TIMESTAMPTZ DEFAULT now(),payment_due_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE plugin_idc_core_credit_repayment(id BIGINT,tenant_id INT,customer_no TEXT,repayment_no TEXT,currency_code TEXT,status_code TEXT,amount_minor BIGINT DEFAULT 100,allocated_amount_minor BIGINT DEFAULT 0,created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE plugin_idc_core_credit_repayment_allocation(id BIGINT,tenant_id INT,repayment_no TEXT,statement_no TEXT)`,
		`INSERT INTO plugin_idc_core_billing_invoice(id,tenant_id,invoice_no,order_no,currency_code,status_code) SELECT i,0,'invoice_'||i,'order_'||i,'CNY','paid' FROM generate_series(1,150) i`,
		`INSERT INTO plugin_idc_core_channel_order_attribution SELECT 0,CASE WHEN i<=125 THEN 'channel_a' ELSE 'channel_b' END,CASE WHEN i<=125 THEN 'payer_a' ELSE 'payer_b' END,'order_'||i FROM generate_series(1,150) i`,
		`INSERT INTO plugin_idc_core_credit_statement(id,tenant_id,customer_no,statement_no,currency_code,status_code) SELECT i,0,CASE WHEN i<=125 THEN 'payer_a' ELSE 'payer_b' END,'statement_'||i,'CNY','unpaid' FROM generate_series(1,150) i`,
		`INSERT INTO plugin_idc_core_credit_repayment(id,tenant_id,customer_no,repayment_no,currency_code,status_code) SELECT i,0,CASE WHEN i<=125 THEN 'payer_a' ELSE 'payer_b' END,'repayment_'||i,'CNY','succeeded' FROM generate_series(1,150) i`,
		`INSERT INTO plugin_idc_core_credit_repayment_allocation VALUES (1,0,'repayment_5','statement_5'),(2,0,'repayment_5','statement_4')`,
	} {
		if _, err = database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &channelCommerceRuntime{database: database}
	page := channelcommercecap.FinancePage{Page: 13, PageSize: 10}
	result, err := runtime.loadChannelFinanceLists(ctx, "channel_a", "payer_a", channelcommercecap.FinancePages{Invoices: page, Statements: page, Repayments: page})
	if err != nil {
		t.Fatal(err)
	}
	if result.InvoiceTotal != 125 || result.StatementTotal != 125 || result.RepaymentTotal != 125 || len(result.Invoices) != 5 || len(result.CreditStatements) != 5 || len(result.CreditRepayments) != 5 {
		t.Fatalf("owner totals/page: %+v", result)
	}
	if result.Invoices[0].InvoiceNo != "invoice_5" || result.CreditStatements[0].StatementNo != "statement_5" || result.CreditRepayments[0].RepaymentNo != "repayment_5" || result.CreditRepayments[0].StatementNo != "statement_5" {
		t.Fatalf("stable order/allocation: %+v", result)
	}
	page.Page = 14
	empty, err := runtime.loadChannelFinanceLists(ctx, "channel_a", "payer_a", channelcommercecap.FinancePages{Invoices: page, Statements: page, Repayments: page})
	if err != nil || empty.InvoiceTotal != 125 || len(empty.Invoices) != 0 || len(empty.CreditRepayments) != 0 {
		t.Fatalf("empty tail retains total: %+v %v", empty, err)
	}
	page = channelcommercecap.FinancePage{Page: 1, PageSize: 100}
	other, err := runtime.loadChannelFinanceLists(ctx, "channel_b", "payer_b", channelcommercecap.FinancePages{Invoices: page, Statements: page, Repayments: page})
	if err != nil || other.InvoiceTotal != 25 || other.StatementTotal != 25 || other.RepaymentTotal != 25 || len(other.Invoices) != 25 {
		t.Fatalf("owner scope: %+v %v", other, err)
	}
	mismatch, err := runtime.loadChannelFinanceLists(ctx, "channel_a", "payer_b", channelcommercecap.FinancePages{Invoices: page, Statements: page, Repayments: page})
	if err != nil || mismatch.InvoiceTotal != 0 {
		t.Fatalf("invoice requires matching channel and payer: %+v %v", mismatch, err)
	}
}
