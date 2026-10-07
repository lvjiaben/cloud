package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
)

type ordinaryTriggerContract struct {
	tableName    string
	legacyName   string
	currentName  string
	functionName string
	clause       string
}

var ordinaryTriggerContracts = []ordinaryTriggerContract{
	{"plugin_idc_core_secret_version", "trg_plugin_linapro_idc_core_secret_version_rotation_fence", "trg_plugin_idc_core_secret_version_rotation_fence", "plugin_idc_core_invalidate_secret_rotation", "AFTER INSERT OR UPDATE OR DELETE"},
	{"plugin_idc_core_secret_key_wrap", "trg_plugin_linapro_idc_core_secret_wrap_rotation_fence", "trg_plugin_idc_core_secret_wrap_rotation_fence", "plugin_idc_core_invalidate_secret_rotation", "AFTER UPDATE OR DELETE"},
	{"plugin_idc_core_business_audit_customer_link", "trg_plugin_linapro_idc_core_validate_audit_customer_link_insert", "trg_plugin_idc_core_validate_audit_customer_link_insert", "plugin_idc_core_validate_audit_customer_link_insert", "AFTER INSERT|REFERENCING NEW TABLE AS inserted_audit_customer_links|FOR EACH STATEMENT"},
	{"plugin_idc_core_business_audit_customer_link", "trg_plugin_linapro_idc_core_reject_audit_customer_link_update", "trg_plugin_idc_core_reject_audit_customer_link_update", "plugin_idc_core_reject_audit_customer_link_mutation", "BEFORE UPDATE|FOR EACH STATEMENT"},
	{"plugin_idc_core_business_audit_customer_link", "trg_plugin_linapro_idc_core_reject_audit_customer_link_delete", "trg_plugin_idc_core_reject_audit_customer_link_delete", "plugin_idc_core_reject_audit_customer_link_mutation", "BEFORE DELETE|FOR EACH STATEMENT"},
	{"plugin_idc_core_financial_internal_note", "trg_plugin_linapro_idc_core_financial_note_immutable", "trg_plugin_idc_core_financial_note_immutable", "plugin_idc_core_reject_financial_note_mutation", "BEFORE UPDATE OR DELETE"},
	{"plugin_idc_core_commerce_order_item", "trg_linapro_idc_core_capture_order_cost", "trg_idc_core_capture_order_cost", "plugin_idc_core_capture_order_cost", "AFTER INSERT"},
}

func TestLegacyOrdinaryTriggersAreConsolidatedPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(constraintTriggerMigrationDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	for _, scenario := range []struct {
		name           string
		includeCurrent bool
		mismatch       bool
	}{
		{name: "old and current definitions coexist", includeCurrent: true},
		{name: "only old definitions exist"},
		{name: "old and current definitions differ", includeCurrent: true, mismatch: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			verifyLegacyOrdinaryTriggerScenario(t, dsn, scenario.includeCurrent, scenario.mismatch)
		})
	}
}

func verifyLegacyOrdinaryTriggerScenario(t *testing.T, dsn string, includeCurrent, mismatch bool) {
	t.Helper()
	ctx := context.Background()
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open ordinary-trigger migration database: %v", err)
	}
	databaseName, err := adminDB.GetValue(ctx, "SELECT current_database()")
	if err != nil || databaseName.String() != "linapro_idc_codegen" {
		_ = adminDB.Close(ctx)
		t.Fatalf("refusing ordinary-trigger test database %q: %v", databaseName.String(), err)
	}
	schema := newConstraintTriggerTestSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteConstraintTriggerIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create ordinary-trigger schema: %v", err)
	}
	t.Cleanup(func() {
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteConstraintTriggerIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop ordinary-trigger schema: %v", dropErr)
		}
		_ = adminDB.Close(ctx)
	})
	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendConstraintTriggerDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped ordinary-trigger database: %v", err)
	}
	t.Cleanup(func() { _ = scopedDB.Close(ctx) })

	createdTables := map[string]bool{}
	createdFunctions := map[string]bool{}
	for _, contract := range ordinaryTriggerContracts {
		if !createdTables[contract.tableName] {
			if _, err = scopedDB.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY)", contract.tableName)); err != nil {
				t.Fatalf("create trigger fixture table %s: %v", contract.tableName, err)
			}
			createdTables[contract.tableName] = true
		}
		if !createdFunctions[contract.functionName] {
			if _, err = scopedDB.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END; $$`, contract.functionName)); err != nil {
				t.Fatalf("create trigger fixture function %s: %v", contract.functionName, err)
			}
			createdFunctions[contract.functionName] = true
		}
		legacyClause := contract.clause
		if mismatch && contract.legacyName == ordinaryTriggerContracts[0].legacyName {
			legacyClause = "AFTER INSERT"
		}
		createOrdinaryTriggerFixture(t, ctx, scopedDB, contract.legacyName, contract.tableName, contract.functionName, legacyClause)
		if includeCurrent {
			createOrdinaryTriggerFixture(t, ctx, scopedDB, contract.currentName, contract.tableName, contract.functionName, contract.clause)
		}
	}

	groups := []struct {
		migration string
		tag       string
		count     int
	}{
		{"002-secret-envelope.sql", "legacy_secret_trigger_names", 2},
		{"009-audit-customer-link.sql", "legacy_audit_customer_trigger_names", 3},
		{"050-financial-operations.sql", "legacy_financial_note_trigger_name", 1},
		{"062-marketing-performance-abuse.sql", "legacy_order_cost_trigger_name", 1},
	}
	if mismatch {
		statements := ordinaryTriggerRefreshStatements(t, groups[0].migration, groups[0].tag, groups[0].count)
		if _, err = scopedDB.Exec(ctx, statements[0]); err == nil {
			t.Fatal("ordinary-trigger cleanup accepted mismatched old and current definitions")
		}
		return
	}
	for _, group := range groups {
		for _, statement := range ordinaryTriggerRefreshStatements(t, group.migration, group.tag, group.count) {
			if _, err = scopedDB.Exec(ctx, statement); err != nil {
				t.Fatalf("execute ordinary-trigger refresh %s: %v", group.migration, err)
			}
		}
	}
	for _, contract := range ordinaryTriggerContracts {
		legacyCount, countErr := scopedDB.GetValue(ctx, "SELECT COUNT(1) FROM pg_trigger WHERE tgrelid = ?::REGCLASS AND tgname = ?", contract.tableName, contract.legacyName)
		if countErr != nil || legacyCount.Int() != 0 {
			t.Fatalf("legacy ordinary trigger %s remains: count=%d err=%v", contract.legacyName, legacyCount.Int(), countErr)
		}
		currentCount, countErr := scopedDB.GetValue(ctx, "SELECT COUNT(1) FROM pg_trigger WHERE tgrelid = ?::REGCLASS AND tgname = ?", contract.tableName, contract.currentName)
		if countErr != nil || currentCount.Int() != 1 {
			t.Fatalf("current ordinary trigger %s missing: count=%d err=%v", contract.currentName, currentCount.Int(), countErr)
		}
	}
}

func createOrdinaryTriggerFixture(t *testing.T, ctx context.Context, database gdb.DB, name, table, function, clause string) {
	t.Helper()
	parts := strings.Split(clause, "|")
	timing := parts[0]
	level := "FOR EACH ROW"
	transition := ""
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "REFERENCING ") {
			transition = part
		} else if strings.HasPrefix(part, "FOR EACH ") {
			level = part
		}
	}
	statement := fmt.Sprintf("CREATE TRIGGER %s %s ON %s %s %s EXECUTE FUNCTION %s()", name, timing, table, transition, level, function)
	if _, err := database.Exec(ctx, statement); err != nil {
		t.Fatalf("create ordinary trigger fixture %s: %v", name, err)
	}
}

func ordinaryTriggerRefreshStatements(t *testing.T, migrationName, tag string, triggerCount int) []string {
	t.Helper()
	migrationPath := filepath.Join(filepath.Dir(auditEvidenceSealingMigrationPath(t)), migrationName)
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read ordinary-trigger migration %s: %v", migrationName, err)
	}
	statements := dialect.SplitSQLStatements(string(migration))
	for index, statement := range statements {
		if strings.Contains(statement, "DO $"+tag+"$") {
			end := index + 1 + triggerCount*2
			if end > len(statements) {
				t.Fatalf("ordinary-trigger migration %s lacks %d refresh statements", migrationName, triggerCount*2)
			}
			return statements[index:end]
		}
	}
	t.Fatalf("ordinary-trigger migration %s lacks cleanup block %s", migrationName, tag)
	return nil
}
