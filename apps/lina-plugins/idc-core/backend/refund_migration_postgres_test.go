// This file verifies the online-refund migration is repeatable on PostgreSQL.

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestOnlineRefundMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL online-refund migration tests", catalogIntegrationDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open online-refund migration database: %v", err)
	}
	suffix := make([]byte, 8)
	if _, err = rand.Read(suffix); err != nil {
		t.Fatalf("generate online-refund schema suffix: %v", err)
	}
	schema := "linapro_refund_test_" + hex.EncodeToString(suffix)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatalf("create online-refund schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_refund_test_") {
			if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop online-refund schema: %v", dropErr)
			}
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close online-refund migration database: %v", closeErr)
		}
	})

	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("open scoped online-refund database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped online-refund database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create online-refund dictionary fixture: %v", err)
	}
	for _, path := range onlineRefundMigrationPaths(t) {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read online-refund migration %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if filepath.Base(path) == "052-online-refund.sql" {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute online-refund migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}

	tables, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema = ? AND table_name IN (
    'plugin_idc_core_refund_policy_version',
    'plugin_idc_core_refund_request',
    'plugin_idc_core_refund_request_item',
    'plugin_idc_core_refund_service_impact',
    'plugin_idc_core_refund_source_allocation',
    'plugin_idc_core_refund_service_lock',
    'plugin_idc_core_refund_decision',
    'plugin_idc_core_refund_execution_command'
)`, schema)
	if err != nil || tables.Int() != 8 {
		t.Fatalf("online-refund table count=%d want=8 err=%v", tables.Int(), err)
	}
	dictionaryTypes, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM sys_dict_type
WHERE tenant_id=0 AND type IN (
    'idc_refund_status', 'idc_refund_reason',
    'idc_refund_approval_mode', 'idc_refund_execution_status'
)`)
	if err != nil || dictionaryTypes.Int() != 4 {
		t.Fatalf("online-refund dictionary count=%d want=4 err=%v", dictionaryTypes.Int(), err)
	}
}

func onlineRefundMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve online-refund migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	names := []string{
		"001-core-foundation.sql",
		"010-catalog-foundation.sql",
		"013-catalog-billing-policy.sql",
		"014-catalog-inventory-routing.sql",
		"019-catalog-tax-policy.sql",
		"022-commerce-foundation.sql",
		"023-billing-ledger-foundation.sql",
		"027-billing-invoice-lifecycle.sql",
		"030-service-renewal-foundation.sql",
		"031-service-renewal-execution.sql",
		"032-service-expiry-scheduling.sql",
		"052-online-refund.sql",
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(root, name))
	}
	return paths
}
