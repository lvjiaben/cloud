// This file verifies persisted lifecycle reads are bounded by the application,
// service, owner and payer together, and never reveal an unrelated operation.
package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestChannelLifecycleReadOwnerIsolationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skip("PostgreSQL lifecycle read DSN is not configured")
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
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_core_business_operation(tenant_id INT,operation_no TEXT,quote_no TEXT,status_code TEXT,currency_code TEXT,subject_type_code TEXT,subject_no TEXT,customer_no TEXT,payer_customer_no TEXT,invoice_no TEXT)`,
		`CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT,order_no TEXT)`,
		`CREATE TABLE plugin_idc_core_channel_order_attribution(tenant_id INT,channel_id TEXT,order_no TEXT,service_owner_customer_no TEXT,payer_customer_no TEXT)`,
		`CREATE TABLE plugin_idc_core_billing_invoice(tenant_id INT,invoice_no TEXT,total_amount_minor BIGINT)`,
		`INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'service_a','order_a'),(0,'service_b','order_b')`,
		`INSERT INTO plugin_idc_core_channel_order_attribution VALUES(0,'app_a','order_a','customer_a','customer_a'),(0,'app_b','order_b','customer_b','customer_b')`,
		`INSERT INTO plugin_idc_core_billing_invoice VALUES(0,'invoice_a',300)`,
		`INSERT INTO plugin_idc_core_business_operation VALUES(0,'operation_a','quote_a','awaiting_financial_settlement','CNY','entitlement','service_a','customer_a','customer_a','invoice_a'),(0,'operation_b',NULL,'completed','CNY','entitlement','service_b','customer_b','customer_b',NULL),(0,'operation_wrong_owner',NULL,'completed','CNY','entitlement','service_a','customer_b','customer_a',NULL)`,
	} {
		if _, err = database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &channelCommerceRuntime{database: database}
	result, err := runtime.GetLifecycle(ctx, "app_a", "service_a", "operation_a")
	if err != nil || result == nil || result.AmountMinor != 300 || !result.PaymentRequired || result.QuoteNo != "quote_a" {
		t.Fatalf("owned operation = %+v, %v", result, err)
	}
	_, hidden := runtime.GetLifecycle(ctx, "app_a", "service_a", "missing")
	if hidden == nil {
		t.Fatal("missing operation visible")
	}
	for _, input := range [][3]string{{"app_b", "service_a", "operation_a"}, {"app_a", "service_b", "operation_b"}, {"app_a", "service_a", "operation_b"}, {"app_a", "service_a", "operation_wrong_owner"}} {
		value, readErr := runtime.GetLifecycle(ctx, input[0], input[1], input[2])
		if value != nil || readErr == nil || readErr.Error() != hidden.Error() {
			t.Fatalf("isolation leaked %+v: %+v %v", input, value, readErr)
		}
	}
	status, err := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_business_operation WHERE operation_no='operation_a'`)
	if err != nil || status.String() != "awaiting_financial_settlement" {
		t.Fatal("GET mutated operation", err)
	}
	for _, statement := range []string{
		`ALTER TABLE plugin_idc_core_business_operation ADD COLUMN operation_type_code TEXT DEFAULT 'renewal', ADD COLUMN idempotency_digest TEXT`,
		`ALTER TABLE plugin_idc_core_service_entitlement ADD COLUMN expires_at TIMESTAMPTZ DEFAULT '2026-11-11T00:00:00Z'`,
	} {
		if _, err = database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	digest := sha256.Sum256([]byte("local-request-1:confirm"))
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET idempotency_digest=? WHERE operation_no='operation_a'`, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
	recovered, err := runtime.ReadRenewalByRequest(ctx, "app_a", "service_a", "local-request-1")
	if err != nil || !recovered.Found || recovered.OperationNo != "operation_a" || recovered.AmountMinor != 300 || recovered.CurrentExpiresAt == nil {
		t.Fatalf("request recovery = %#v %v", recovered, err)
	}
	for _, query := range [][3]string{{"app_b", "service_a", "local-request-1"}, {"app_a", "service_b", "local-request-1"}, {"app_a", "service_a", "local-request-2"}} {
		recovered, err = runtime.ReadRenewalByRequest(ctx, query[0], query[1], query[2])
		if err != nil || recovered.Found || recovered.OperationNo != "" {
			t.Fatalf("request recovery isolation = %#v %v", recovered, err)
		}
	}

}
