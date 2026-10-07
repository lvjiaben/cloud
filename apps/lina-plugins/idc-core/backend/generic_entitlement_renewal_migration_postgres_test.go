package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestGenericEntitlementRenewalMigrationPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL entitlement renewal migration tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open entitlement renewal migration database: %v", err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatalf("create isolated entitlement renewal schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		_ = adminDB.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatalf("open isolated entitlement renewal database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create dictionary fixtures: %v", err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve entitlement renewal migration path")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "*.sql"))
	if err != nil {
		t.Fatalf("list entitlement renewal prerequisites: %v", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(path), readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute migration %s: %v", filepath.Base(path), err)
		}
	}
	renewalMigration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "088-generic-entitlement-renewal.sql"))
	if err != nil {
		t.Fatalf("read renewal replay migration: %v", err)
	}
	if _, err = database.Exec(ctx, string(renewalMigration)); err != nil {
		t.Fatalf("replay entitlement renewal migration: %v", err)
	}

	oldColumnCount, err := database.GetValue(ctx, `SELECT count(*) FROM information_schema.columns
WHERE table_schema=? AND table_name IN
('plugin_idc_core_service_renewal_policy','plugin_idc_core_service_renewal_quote_request',
 'plugin_idc_core_service_renewal_attempt')
AND column_name IN ('service_no','service_version','expected_service_version')`, schema)
	if err != nil {
		t.Fatalf("inspect legacy renewal columns: %v", err)
	}
	if oldColumnCount.Int() != 0 {
		t.Fatalf("renewal schema still exposes %d legacy service columns", oldColumnCount.Int())
	}
}
