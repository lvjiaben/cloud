// This file verifies the three-level contract policy schema and prepares the
// explicitly named disposable DAO code-generation database.

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

func TestContractPolicyMigrationPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL contract policy migration tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create contract policy migration admin database: %v", err)
	}
	schema := newCommerceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated contract policy schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_commerce_test_") {
			if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop isolated contract policy schema: %v", dropErr)
			}
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close contract policy migration admin database: %v", closeErr)
		}
	})
	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped contract policy database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped contract policy database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create contract policy dictionary fixtures: %v", err)
	}
	paths := contractPolicyMigrationPaths(t)
	for index, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read contract policy migration %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if index == len(paths)-1 {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute contract policy migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}
	validated, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM pg_constraint c
JOIN pg_namespace n ON n.oid = c.connamespace
WHERE conname IN (
  'ck_idc_customer_enterprise_contract_policy',
  'ck_idc_catalog_version_contract_policy',
  'ck_idc_com_quote_item_contract_policy',
  'ck_idc_com_order_item_contract_policy',
  'ck_idc_com_assisted_contract_policy',
  'ck_idc_com_order_contract_policy'
) AND convalidated = TRUE AND n.nspname = ?`, schema)
	if err != nil || validated.Int() != 6 {
		t.Fatalf("validated contract policy constraints=%d want=6 err=%v", validated.Int(), err)
	}
	columns, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = ? AND column_name IN (
  'enterprise_contract_policy_code', 'enterprise_contract_template_no',
  'contract_policy_code', 'contract_template_no', 'contract_policy_version',
  'contract_override_policy_code', 'contract_override_template_no',
  'contract_decision_source_code', 'contract_template_version_no',
  'contract_template_name', 'contract_language_code', 'contract_scope_code',
  'contract_signing_order_code'
)`, schema)
	if err != nil || columns.Int() != 22 {
		t.Fatalf("contract policy columns=%d want=22 err=%v", columns.Int(), err)
	}
}

func TestContractPolicyCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare contract policy DAO metadata", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create contract policy code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close contract policy code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	migration, err := os.ReadFile(contractPolicyMigrationPaths(t)[7])
	if err != nil {
		t.Fatalf("read contract policy code-generation migration: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute contract policy code-generation migration attempt %d: %v", attempt+1, err)
		}
	}
}

func contractPolicyMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract policy migration test path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	return []string{
		filepath.Join(migrationRoot, "001-core-foundation.sql"),
		filepath.Join(migrationRoot, "010-catalog-foundation.sql"),
		filepath.Join(migrationRoot, "013-catalog-billing-policy.sql"),
		filepath.Join(migrationRoot, "014-catalog-inventory-routing.sql"),
		filepath.Join(migrationRoot, "019-catalog-tax-policy.sql"),
		filepath.Join(migrationRoot, "022-commerce-foundation.sql"),
		filepath.Join(migrationRoot, "026-commerce-assisted-order.sql"),
		filepath.Join(migrationRoot, "043-contract-policy.sql"),
	}
}
