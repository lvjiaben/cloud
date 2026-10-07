package backend

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProviderCatalogDraftRetirementMigrationIsFailClosedAndIdempotent(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve retirement migration test path")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "089-retire-provider-catalog-draft.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retirement migration: %v", err)
	}
	sql := string(content)
	for _, required := range []string{
		`WHERE "source_code" = 'provider_import'`,
		"RAISE EXCEPTION",
		"rebuild this development database from current migrations",
		"DROP TABLE IF EXISTS plugin_idc_core_catalog_provider_draft",
		"DROP CONSTRAINT IF EXISTS ck_idc_catalog_version_source",
		`"source_code" IN ('manual', 'migration', 'copy')`,
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("retirement migration is missing %q", required)
		}
	}
}
