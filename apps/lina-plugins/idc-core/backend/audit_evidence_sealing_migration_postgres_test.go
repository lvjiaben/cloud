// This file verifies deferred constraint-trigger migrations remain safe when
// source-plugin upgrades replay install SQL inside one PostgreSQL transaction.

package backend

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
)

const constraintTriggerMigrationDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

// TestConstraintTriggerMigrationsUseTransactionSafeExistenceGuards scans every
// idc-core install migration so a later constraint trigger cannot reintroduce
// the same transactional DROP-and-recreate failure as the audit seal.
func TestConstraintTriggerMigrationsUseTransactionSafeExistenceGuards(t *testing.T) {
	migrationRoot := filepath.Dir(auditEvidenceSealingMigrationPath(t))
	paths, err := filepath.Glob(filepath.Join(migrationRoot, "*.sql"))
	if err != nil {
		t.Fatalf("scan idc-core migrations: %v", err)
	}
	createPattern := regexp.MustCompile(`(?i)CREATE\s+CONSTRAINT\s+TRIGGER\s+([a-zA-Z0-9_]+)`)
	dropThenCreatePattern := regexp.MustCompile(`(?is)DROP\s+TRIGGER\s+IF\s+EXISTS\s+([a-zA-Z0-9_]+).*?CREATE\s+CONSTRAINT\s+TRIGGER\s+([a-zA-Z0-9_]+)`)
	constraintBlockPattern := regexp.MustCompile(`(?s)DO \$constraint_trigger\$.*?\$constraint_trigger\$;`)
	constraintTriplePattern := regexp.MustCompile(`(?is)CREATE CONSTRAINT TRIGGER\s+([a-zA-Z0-9_]+)\s+.*?\sON\s+([a-zA-Z0-9_]+).*?EXECUTE FUNCTION\s+([a-zA-Z0-9_]+)\(\);`)
	ordinaryTriggerPattern := regexp.MustCompile(`(?im)^\s*(?:DROP\s+TRIGGER|CREATE\s+TRIGGER)\b`)
	total := 0
	triples := make([]string, 0, 49)
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read idc-core migration %s: %v", filepath.Base(path), readErr)
		}
		text := string(content)
		matches := createPattern.FindAllStringSubmatch(text, -1)
		if len(matches) == 0 {
			continue
		}
		for _, unsafe := range dropThenCreatePattern.FindAllStringSubmatch(text, -1) {
			if unsafe[1] == unsafe[2] {
				t.Fatalf("migration %s still drops and recreates constraint trigger %s in one transaction", filepath.Base(path), unsafe[1])
			}
		}
		for _, match := range matches {
			name := match[1]
			if !strings.Contains(text, "conname = '"+name+"'") {
				t.Fatalf("migration %s constraint trigger %s lacks a pg_constraint identity guard", filepath.Base(path), name)
			}
			total++
		}
		for _, block := range constraintBlockPattern.FindAllString(text, -1) {
			if ordinaryTriggerPattern.MatchString(block) {
				t.Fatalf("migration %s constraint-trigger guard contains an ordinary trigger statement", filepath.Base(path))
			}
			triple := constraintTriplePattern.FindStringSubmatch(block)
			if len(triple) != 4 {
				t.Fatalf("migration %s constraint-trigger guard lacks one parseable name/table/function triple", filepath.Base(path))
			}
			name, table, functionName := triple[1], triple[2], triple[3]
			if strings.Count(block, "CREATE CONSTRAINT TRIGGER") != 1 || strings.Count(block, "EXECUTE FUNCTION") != 1 {
				t.Fatalf("migration %s constraint-trigger guard %s contains extra trigger statements", filepath.Base(path), name)
			}
			for _, expected := range []string{
				"constraint_row.conrelid = '" + table + "'::regclass",
				"constraint_row.conname = '" + name + "'",
				"trigger_row.tgname = '" + name + "'",
				"trigger_row.tgfoid = '" + functionName + "()'::regprocedure",
			} {
				if !strings.Contains(block, expected) {
					t.Fatalf("migration %s constraint-trigger triple %s|%s|%s lacks guard %q", filepath.Base(path), name, table, functionName, expected)
				}
			}
			triples = append(triples, name+"|"+table+"|"+functionName)
		}
	}
	// Migration 087 replaces five separate deferred refund-composition validators
	// with five ordinary enqueue triggers and one transaction/refund-deduplicated
	// deferred queue validator. The current constraint-trigger inventory is
	// therefore four entries smaller while preserving all five mutation sources.
	assertRefundConstraintTriggerDeduplication(t, migrationRoot)
	// Migration 110 adds the deferred catalog-import draft-binding invariant.
	if total != 65 {
		t.Fatalf("expected to govern 65 current idc-core constraint triggers including catalog-import draft binding, got %d", total)
	}
	sort.Strings(triples)
	digest := sha256.Sum256([]byte(strings.Join(triples, "\n")))
	const expectedTripleDigest = "35143f9464daa5b18d4ff169017097bbc0a0e02b6ff7aeeba59d15ca74c575f7"
	if got := hex.EncodeToString(digest[:]); got != expectedTripleDigest {
		t.Fatalf("constraint-trigger name/table/function inventory changed: digest=%s triples=%q", got, triples)
	}
}

func assertRefundConstraintTriggerDeduplication(t *testing.T, migrationRoot string) {
	t.Helper()
	refundMigration, err := os.ReadFile(filepath.Join(migrationRoot, "087-generic-refund-entitlement.sql"))
	if err != nil {
		t.Fatalf("read generic refund migration: %v", err)
	}
	refundText := string(refundMigration)
	for _, sourceTrigger := range []string{
		"trg_idc_refund_request_composition",
		"trg_idc_refund_item_composition",
		"trg_idc_refund_source_composition",
		"trg_idc_refund_impact_composition",
		"trg_idc_refund_lock_composition",
	} {
		if !strings.Contains(refundText, "CREATE TRIGGER "+sourceTrigger) {
			t.Fatalf("generic refund migration lacks enqueue trigger %s", sourceTrigger)
		}
	}
	if got := strings.Count(refundText,
		"FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_refund_composition_validation()"); got != 5 {
		t.Fatalf("generic refund migration enqueue trigger count=%d want=5", got)
	}
	if got := strings.Count(refundText, "CREATE CONSTRAINT TRIGGER"); got != 1 ||
		!strings.Contains(refundText,
			"EXECUTE FUNCTION plugin_idc_core_run_queued_refund_composition_validation()") {
		t.Fatalf("generic refund migration deferred queue validator is not exactly one: %d", got)
	}
}

func TestFinancialSettlementOrdinaryTriggersRemainOutsideConstraintGuard(t *testing.T) {
	migrationPath := filepath.Join(filepath.Dir(auditEvidenceSealingMigrationPath(t)), "035-financial-settlement.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read financial settlement migration: %v", err)
	}
	statements := dialect.SplitSQLStatements(string(migration))
	expectedIndependentStatements := []string{
		"DROP TRIGGER IF EXISTS trg_idc_fin_source_validate",
		"CREATE TRIGGER trg_idc_fin_source_validate",
		"DROP TRIGGER IF EXISTS trg_idc_fin_settle_immutable",
		"CREATE TRIGGER trg_idc_fin_settle_immutable",
		"DROP TRIGGER IF EXISTS trg_idc_fin_source_immutable",
		"CREATE TRIGGER trg_idc_fin_source_immutable",
	}
	for _, expected := range expectedIndependentStatements {
		found := false
		for _, statement := range statements {
			if strings.HasPrefix(strings.TrimSpace(statement), expected) {
				found = true
				if strings.Contains(statement, "DO $constraint_trigger$") {
					t.Fatalf("ordinary trigger statement %q remains inside the constraint guard", expected)
				}
				break
			}
		}
		if !found {
			t.Fatalf("production SQL splitter did not expose independent statement %q", expected)
		}
	}
}

// TestAuditEvidenceSealingMigrationReplaysInOneTransactionPostgreSQL creates
// the historical trigger and constraint first, then replays 005 through the
// same statement splitter and transaction boundary used by plugin upgrades.
func TestAuditEvidenceSealingMigrationReplaysInOneTransactionPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(constraintTriggerMigrationDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open constraint-trigger integration database: %v", err)
	}
	databaseName, err := adminDB.GetValue(ctx, "SELECT current_database()")
	if err != nil || databaseName.String() != "linapro_idc_codegen" {
		_ = adminDB.Close(ctx)
		t.Fatalf("refusing constraint-trigger test database %q: %v", databaseName.String(), err)
	}
	schema := newConstraintTriggerTestSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteConstraintTriggerIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create constraint-trigger schema: %v", err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "linapro_constraint_trigger_test_") {
			t.Errorf("refusing to drop unexpected constraint-trigger schema %q", schema)
		} else if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteConstraintTriggerIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop constraint-trigger schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close constraint-trigger integration database: %v", closeErr)
		}
	})
	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendConstraintTriggerDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatalf("open scoped constraint-trigger database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := scopedDB.Close(ctx); closeErr != nil {
			t.Errorf("close scoped constraint-trigger database: %v", closeErr)
		}
	})
	if _, err = scopedDB.Exec(ctx, `
CREATE TABLE plugin_idc_core_business_audit_event (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    context_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB
);
CREATE TABLE plugin_idc_core_business_number (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reservation_scope_digest CHAR(64) NOT NULL DEFAULT '',
    idempotency_key_digest CHAR(64) NOT NULL DEFAULT ''
);`); err != nil {
		t.Fatalf("create audit sealing fixtures: %v", err)
	}
	migration, err := os.ReadFile(auditEvidenceSealingMigrationPath(t))
	if err != nil {
		t.Fatalf("read audit evidence sealing migration: %v", err)
	}
	executeConstraintTriggerMigration(t, ctx, scopedDB, string(migration))
	if _, err = scopedDB.Exec(ctx, `
DROP TRIGGER trg_plugin_idc_core_require_sealed_audit_event
ON plugin_idc_core_business_audit_event;
CREATE CONSTRAINT TRIGGER trg_plugin_linapro_idc_core_require_sealed_audit_event
AFTER INSERT OR UPDATE OR DELETE
ON plugin_idc_core_business_audit_event
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION plugin_idc_core_require_sealed_audit_event();
ALTER TABLE plugin_idc_core_business_audit_event
RENAME CONSTRAINT trg_plugin_linapro_idc_core_require_sealed_audit_event
TO trg_plugin_idc_core_require_sealed_audit_event;`); err != nil {
		t.Fatalf("create historical trigger/constraint name mismatch: %v", err)
	}
	catalogNames, err := scopedDB.GetValue(ctx, `
SELECT constraint_row.conname || ':' || trigger_row.tgname
  FROM pg_constraint constraint_row
  JOIN pg_trigger trigger_row ON trigger_row.tgconstraint = constraint_row.oid
 WHERE constraint_row.conrelid = 'plugin_idc_core_business_audit_event'::regclass
   AND constraint_row.conname = 'trg_plugin_idc_core_require_sealed_audit_event'
   AND constraint_row.contype = 't'`)
	if err != nil {
		t.Fatalf("read historical trigger/constraint name mismatch: %v", err)
	}
	const expectedCatalogNames = "trg_plugin_idc_core_require_sealed_audit_event:trg_plugin_linapro_idc_core_require_sealed_audit_event"
	if catalogNames.String() != expectedCatalogNames {
		t.Fatalf("expected historical catalog mismatch %q, got %q", expectedCatalogNames, catalogNames.String())
	}
	catalogIdentityBefore, err := scopedDB.GetValue(ctx, `
SELECT constraint_row.oid::TEXT || ':' || trigger_row.oid::TEXT
  FROM pg_constraint constraint_row
  JOIN pg_trigger trigger_row ON trigger_row.tgconstraint = constraint_row.oid
 WHERE constraint_row.conrelid = 'plugin_idc_core_business_audit_event'::regclass
   AND constraint_row.conname = 'trg_plugin_idc_core_require_sealed_audit_event'
   AND constraint_row.contype = 't'`)
	if err != nil || catalogIdentityBefore.String() == "" {
		t.Fatalf("read historical trigger/constraint identity: value=%q err=%v", catalogIdentityBefore.String(), err)
	}
	executeConstraintTriggerMigration(t, ctx, scopedDB, string(migration))
	executeConstraintTriggerMigration(t, ctx, scopedDB, string(migration))
	count, err := scopedDB.GetValue(ctx, `
SELECT COUNT(1)
  FROM pg_trigger trigger_row
  JOIN pg_constraint constraint_row ON constraint_row.oid = trigger_row.tgconstraint
 WHERE trigger_row.tgrelid = 'plugin_idc_core_business_audit_event'::regclass
   AND NOT trigger_row.tgisinternal
   AND trigger_row.tgname = 'trg_plugin_idc_core_require_sealed_audit_event'
   AND trigger_row.tgfoid = 'plugin_idc_core_require_sealed_audit_event()'::regprocedure
   AND constraint_row.conname = 'trg_plugin_idc_core_require_sealed_audit_event'
   AND constraint_row.contype = 't'`)
	if err != nil {
		t.Fatalf("count replayed audit constraint trigger: %v", err)
	}
	if count.Int() != 1 {
		t.Fatalf("expected one audit constraint trigger and constraint after replay, got %d", count.Int())
	}
	catalogIdentityAfter, err := scopedDB.GetValue(ctx, `
SELECT constraint_row.oid::TEXT || ':' || trigger_row.oid::TEXT
  FROM pg_constraint constraint_row
  JOIN pg_trigger trigger_row ON trigger_row.tgconstraint = constraint_row.oid
 WHERE constraint_row.conrelid = 'plugin_idc_core_business_audit_event'::regclass
   AND constraint_row.conname = 'trg_plugin_idc_core_require_sealed_audit_event'
   AND constraint_row.contype = 't'
   AND trigger_row.tgname = 'trg_plugin_idc_core_require_sealed_audit_event'`)
	if err != nil || catalogIdentityAfter.String() != catalogIdentityBefore.String() {
		t.Fatalf("audit trigger rename changed catalog identity: before=%q after=%q err=%v", catalogIdentityBefore.String(), catalogIdentityAfter.String(), err)
	}
	legacyCount, err := scopedDB.GetValue(ctx, `
SELECT COUNT(1)
  FROM pg_trigger
 WHERE tgrelid = 'plugin_idc_core_business_audit_event'::regclass
   AND tgname = 'trg_plugin_linapro_idc_core_require_sealed_audit_event'`)
	if err != nil || legacyCount.Int() != 0 {
		t.Fatalf("legacy audit trigger name remains after replay: count=%d err=%v", legacyCount.Int(), err)
	}
}

func executeConstraintTriggerMigration(t *testing.T, ctx context.Context, database gdb.DB, migration string) {
	t.Helper()
	err := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		for _, statement := range dialect.SplitSQLStatements(migration) {
			if _, execErr := database.Exec(txCtx, statement); execErr != nil {
				return execErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("replay audit constraint-trigger migration transactionally: %v", err)
	}
}

func auditEvidenceSealingMigrationPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve audit evidence sealing migration path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "005-audit-evidence-sealing.sql"))
}

func newConstraintTriggerTestSchema(t *testing.T) string {
	t.Helper()
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatalf("generate constraint-trigger schema: %v", err)
	}
	return "linapro_constraint_trigger_test_" + hex.EncodeToString(entropy[:])
}

func appendConstraintTriggerDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func quoteConstraintTriggerIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
