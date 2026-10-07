package customer360summary

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestCoreContributorUsesOneCompleteSetQueryPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run customer 360 PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	var token [8]byte
	if _, err = rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_customer_360_test_" + hex.EncodeToString(token[:])
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_customer_360_test_") {
			_, _ = admin.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
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
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_core_commerce_order(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_invoice(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_ledger_account(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,account_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40))`,
		`CREATE TABLE plugin_idc_core_billing_ledger_entry(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,account_no VARCHAR(40) NOT NULL,transaction_no VARCHAR(40) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_support_ticket(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_business_audit_event(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL)`,
	} {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	contributor, err := NewCoreContributor(database)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range contributor.Codes() {
		if code == "servers" {
			t.Fatal("idc-core must not claim the server-plugin summary")
		}
	}
	items, err := contributor.BatchCount(context.Background(), []string{"CUS_missing_one", "CUS_missing_two"})
	if err != nil || len(items) != 10 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	for _, item := range items {
		if item.Total != 0 || item.CustomerNo == "" || item.Code == "" {
			t.Fatalf("unexpected empty customer summary: %+v", item)
		}
	}
}
