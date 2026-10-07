package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestGenericBatchRenewalRetirementProtectsEvidencePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL batch-renewal retirement tests", catalogIntegrationDSNEnv)
	}
	ctx := context.Background()
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open batch-renewal retirement database: %v", err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatalf("create isolated batch-renewal retirement schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		_ = adminDB.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatalf("open isolated batch-renewal retirement database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve batch-renewal retirement migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "095-retire-generic-batch-renewal.sql"))
	if err != nil {
		t.Fatalf("read batch-renewal retirement migration: %v", err)
	}
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_financial_batch_renewal ("id" BIGINT PRIMARY KEY);
CREATE TABLE plugin_idc_core_financial_batch_renewal_item ("id" BIGINT PRIMARY KEY);
INSERT INTO plugin_idc_core_financial_batch_renewal ("id") VALUES (1);
INSERT INTO plugin_idc_core_financial_batch_renewal_item ("id") VALUES (1);`); err != nil {
		t.Fatalf("seed invalid generic batch-renewal evidence: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err == nil || !strings.Contains(err.Error(), "export the development evidence") {
		t.Fatalf("retirement must preserve non-empty development evidence: %v", err)
	}
	if _, err = database.Exec(ctx, `TRUNCATE TABLE plugin_idc_core_financial_batch_renewal_item, plugin_idc_core_financial_batch_renewal`); err != nil {
		t.Fatalf("clear exported development evidence fixture: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("retire empty generic batch-renewal storage: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("replay generic batch-renewal retirement: %v", err)
	}
	for _, table := range []string{"plugin_idc_core_financial_batch_renewal_item", "plugin_idc_core_financial_batch_renewal"} {
		value, queryErr := database.GetValue(ctx, `SELECT to_regclass(?) IS NULL`, table)
		if queryErr != nil || !value.Bool() {
			t.Fatalf("retired table %s still exists: %v", table, queryErr)
		}
	}
}
