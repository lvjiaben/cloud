// This file verifies the peer mapping upgrade and complete SQL replay remain
// idempotent without changing existing non-peer entitlement identity.

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

func TestPeerServiceMappingMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tables, e := admin.GetArray(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=?`, schema)
		if e != nil {
			t.Error(e)
		}
		// Isolated test schemas contain many tables; bounded drops avoid the
		// development server's shared-lock limit for a single huge CASCADE.
		for _, table := range tables {
			if _, e = admin.Exec(ctx, "DROP TABLE IF EXISTS "+quoteCatalogIdentifier(schema)+"."+quoteCatalogIdentifier(table.String())+" CASCADE"); e != nil {
				t.Error(e)
			}
		}
		if _, e = admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); e != nil {
			t.Error(e)
		}
		if e = admin.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := db.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	if _, err = db.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source path unavailable")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	// Upgrade the complete schema, then replay this business change's migrations
	// and the affected backfill twice. Earlier 031 cannot replay after 086 removes legacy columns.
	replay := map[string]bool{
		"090-generic-delivery-entitlement-backfill.sql": true,
		"114-assisted-purchase-configuration.sql":       true,
		"115-direct-assisted-orders.sql":                true,
		"116-sales-management-pricing.sql":              true,
		"118-commission-business-loop.sql":              true,
		"119-sales-payment-facts.sql":                   true,
		"120-portable-catalog-import.sql":               true,
		"121-peer-service-mapping.sql":                  true,
		"122-peer-purchase-files.sql":                   true,
	}
	for pass := range 3 {
		for _, path := range paths {
			if pass > 0 && !replay[filepath.Base(path)] {
				continue
			}
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = db.Exec(ctx, string(raw)); e != nil {
				t.Fatalf("pass %d migration %s: %v", pass+1, filepath.Base(path), e)
			}
		}
	}
	count, err := db.GetValue(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=? AND table_name='plugin_idc_core_service_entitlement' AND column_name IN ('upstream_order_reference','upstream_delivery_no','upstream_service_no')`, schema)
	if err != nil || count.Int() != 3 {
		t.Fatalf("mapping columns: %v %v", count, err)
	}
	index, err := db.GetValue(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname=? AND indexname='uk_plugin_idc_core_entitlement_upstream_service'`, schema)
	if err != nil || index.Int() != 1 {
		t.Fatalf("precise identity index: %v %v", index, err)
	}
}
