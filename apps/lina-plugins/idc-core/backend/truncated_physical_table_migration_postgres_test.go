package backend

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

type truncatedPhysicalTableCase struct {
	name           string
	migration      string
	blockTag       string
	legacyTable    string
	currentTable   string
	fixtureDDL     string
	constraintName string
	evidenceColumn string
	evidenceValue  string
}

func TestTruncatedPhysicalTablesAreRenamedWithoutLosingEvidencePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(constraintTriggerMigrationDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	testCases := []truncatedPhysicalTableCase{
		{
			name:         "support custom field option snapshot",
			migration:    "042-support-content.sql",
			blockTag:     "support_custom_field_option_table",
			legacyTable:  "plugin_idc_core_support_ticket_custom_field_value_optio",
			currentTable: "plugin_idc_core_support_ticket_custom_field_value_option",
			fixtureDDL: `
CREATE TABLE plugin_idc_core_support_ticket_custom_field_value_optio (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 0,
    ticket_no VARCHAR(40) NOT NULL,
    field_no VARCHAR(40) NOT NULL,
    option_no VARCHAR(40) NOT NULL,
    name_zh_cn VARCHAR(128) NOT NULL,
    name_en_us VARCHAR(128) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_custom_field_option UNIQUE (ticket_no, field_no, option_no)
);
INSERT INTO plugin_idc_core_support_ticket_custom_field_value_optio
    (ticket_no, field_no, option_no, name_zh_cn, name_en_us)
VALUES ('ticket-evidence', 'field-evidence', 'option-evidence', '保留值', 'Preserved');`,
			constraintName: "uk_idc_support_ticket_custom_field_option",
			evidenceColumn: "ticket_no",
			evidenceValue:  "ticket-evidence",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verifyTruncatedPhysicalTableRename(t, dsn, testCase)
		})
	}
}

func TestSSHPublicKeyTruncatedObjectsAreConsolidatedPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(constraintTriggerMigrationDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open SSH object migration database: %v", err)
	}
	databaseName, err := adminDB.GetValue(ctx, "SELECT current_database()")
	if err != nil || databaseName.String() != "linapro_idc_codegen" {
		_ = adminDB.Close(ctx)
		t.Fatalf("refusing SSH object test database %q: %v", databaseName.String(), err)
	}
	schema := newConstraintTriggerTestSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteConstraintTriggerIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create SSH object test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteConstraintTriggerIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop SSH object test schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close SSH object migration database: %v", closeErr)
		}
	})
	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendConstraintTriggerDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped SSH object database: %v", err)
	}
	t.Cleanup(func() { _ = scopedDB.Close(ctx) })
	if _, err = scopedDB.Exec(ctx, `
CREATE TABLE plugin_idc_core_customer_ssh_public_key (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 0,
    customer_no VARCHAR(40) NOT NULL,
    fingerprint_sha256 VARCHAR(128) NOT NULL,
    idempotency_key_digest CHAR(64) NOT NULL,
    request_fingerprint CHAR(64) NOT NULL,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_idempotency_
        CHECK (idempotency_key_digest ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_request_fing
        CHECK (request_fingerprint ~ '^[0-9a-f]{64}$')
);
CREATE UNIQUE INDEX uq_plugin_idc_core_customer_ssh_public_key_active_finge
    ON plugin_idc_core_customer_ssh_public_key (tenant_id, customer_no, fingerprint_sha256)
    WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX uq_plugin_idc_core_customer_ssh_public_key_create_reque
    ON plugin_idc_core_customer_ssh_public_key (tenant_id, customer_no, idempotency_key_digest);
INSERT INTO plugin_idc_core_customer_ssh_public_key
    (customer_no, fingerprint_sha256, idempotency_key_digest, request_fingerprint)
VALUES ('customer-evidence', 'SHA256:evidence', repeat('a', 64), repeat('b', 64));`); err != nil {
		t.Fatalf("create SSH truncated object fixture: %v", err)
	}

	migration, err := os.ReadFile(filepath.Join(filepath.Dir(auditEvidenceSealingMigrationPath(t)), "063-customer-ssh-public-key.sql"))
	if err != nil {
		t.Fatalf("read SSH public key migration: %v", err)
	}
	constraintBlock := taggedMigrationBlock(t, migration, "ssh_public_key_constraint_names")
	indexBlock := taggedMigrationBlock(t, migration, "ssh_public_key_index_names")
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = scopedDB.Exec(ctx, constraintBlock); err != nil {
			t.Fatalf("consolidate SSH constraints attempt %d: %v", attempt+1, err)
		}
		if _, err = scopedDB.Exec(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_idc_core_customer_ssh_public_key_active_fingerprint
    ON plugin_idc_core_customer_ssh_public_key (tenant_id, customer_no, fingerprint_sha256)
    WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_idc_core_customer_ssh_public_key_create_request
    ON plugin_idc_core_customer_ssh_public_key (tenant_id, customer_no, idempotency_key_digest);`); err != nil {
			t.Fatalf("create current SSH indexes attempt %d: %v", attempt+1, err)
		}
		if _, err = scopedDB.Exec(ctx, indexBlock); err != nil {
			t.Fatalf("consolidate SSH indexes attempt %d: %v", attempt+1, err)
		}
	}

	legacyObjects, err := scopedDB.GetValue(ctx, `
SELECT
    (SELECT COUNT(1) FROM pg_constraint
      WHERE conrelid = 'plugin_idc_core_customer_ssh_public_key'::REGCLASS
        AND conname IN ('ck_plugin_idc_core_customer_ssh_public_key_idempotency_',
                        'ck_plugin_idc_core_customer_ssh_public_key_request_fing'))
  + (SELECT COUNT(1) FROM pg_class
      WHERE relnamespace = current_schema()::REGNAMESPACE
        AND relname IN ('uq_plugin_idc_core_customer_ssh_public_key_active_finge',
                        'uq_plugin_idc_core_customer_ssh_public_key_create_reque'))`)
	if err != nil || legacyObjects.Int() != 0 {
		t.Fatalf("legacy SSH objects remain: count=%d err=%v", legacyObjects.Int(), err)
	}
	currentObjects, err := scopedDB.GetValue(ctx, `
SELECT
    (SELECT COUNT(1) FROM pg_constraint
      WHERE conrelid = 'plugin_idc_core_customer_ssh_public_key'::REGCLASS
        AND conname IN ('ck_plugin_idc_core_customer_ssh_public_key_idempotency_digest',
                        'ck_plugin_idc_core_customer_ssh_public_key_request_fingerprint'))
  + (SELECT COUNT(1) FROM pg_class
      WHERE relnamespace = current_schema()::REGNAMESPACE
        AND relname IN ('uq_plugin_idc_core_customer_ssh_public_key_active_fingerprint',
                        'uq_plugin_idc_core_customer_ssh_public_key_create_request'))`)
	if err != nil || currentObjects.Int() != 4 {
		t.Fatalf("current SSH objects missing: count=%d err=%v", currentObjects.Int(), err)
	}
	evidence, err := scopedDB.GetValue(ctx, "SELECT customer_no FROM plugin_idc_core_customer_ssh_public_key")
	if err != nil || evidence.String() != "customer-evidence" {
		t.Fatalf("SSH evidence changed during object consolidation: value=%q err=%v", evidence.String(), err)
	}
	if _, err = scopedDB.Exec(ctx, `
CREATE UNIQUE INDEX uq_plugin_idc_core_customer_ssh_public_key_active_finge
    ON plugin_idc_core_customer_ssh_public_key (tenant_id, customer_no, fingerprint_sha256)
    WHERE revoked_at IS NOT NULL;`); err != nil {
		t.Fatalf("create mismatched legacy SSH index: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, indexBlock); err == nil {
		t.Fatal("SSH object cleanup accepted a mismatched legacy index")
	}
}

func taggedMigrationBlock(t *testing.T, migration []byte, tag string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)DO \$` + regexp.QuoteMeta(tag) + `\$.*?\$` + regexp.QuoteMeta(tag) + `\$;`)
	block := pattern.Find(migration)
	if len(block) == 0 {
		t.Fatalf("migration lacks tagged block %s", tag)
	}
	return string(block)
}

func verifyTruncatedPhysicalTableRename(t *testing.T, dsn string, testCase truncatedPhysicalTableCase) {
	t.Helper()
	ctx := context.Background()
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open truncated-table integration database: %v", err)
	}
	databaseName, err := adminDB.GetValue(ctx, "SELECT current_database()")
	if err != nil || databaseName.String() != "linapro_idc_codegen" {
		_ = adminDB.Close(ctx)
		t.Fatalf("refusing truncated-table test database %q: %v", databaseName.String(), err)
	}
	schema := newConstraintTriggerTestSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteConstraintTriggerIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create truncated-table schema: %v", err)
	}
	t.Cleanup(func() {
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteConstraintTriggerIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop truncated-table schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close truncated-table integration database: %v", closeErr)
		}
	})
	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendConstraintTriggerDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped truncated-table database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := scopedDB.Close(ctx); closeErr != nil {
			t.Errorf("close scoped truncated-table database: %v", closeErr)
		}
	})
	if _, err = scopedDB.Exec(ctx, testCase.fixtureDDL); err != nil {
		t.Fatalf("create truncated-table fixture: %v", err)
	}
	legacyOID, err := scopedDB.GetValue(ctx, "SELECT to_regclass(?)::oid", testCase.legacyTable)
	if err != nil || legacyOID.Int64() == 0 {
		t.Fatalf("read truncated-table fixture oid: oid=%d err=%v", legacyOID.Int64(), err)
	}

	migrationPath := filepath.Join(filepath.Dir(auditEvidenceSealingMigrationPath(t)), testCase.migration)
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read truncated-table migration: %v", err)
	}
	blockPattern := regexp.MustCompile(`(?s)DO \$` + regexp.QuoteMeta(testCase.blockTag) + `\$.*?\$` + regexp.QuoteMeta(testCase.blockTag) + `\$;`)
	block := blockPattern.Find(migration)
	if len(block) == 0 {
		t.Fatalf("migration %s lacks %s block", testCase.migration, testCase.blockTag)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = scopedDB.Exec(ctx, string(block)); err != nil {
			t.Fatalf("execute truncated-table rename attempt %d: %v", attempt+1, err)
		}
	}

	currentOID, err := scopedDB.GetValue(ctx, "SELECT to_regclass(?)::oid", testCase.currentTable)
	if err != nil || currentOID.Int64() != legacyOID.Int64() {
		t.Fatalf("table identity changed during rename: before=%d after=%d err=%v", legacyOID.Int64(), currentOID.Int64(), err)
	}
	legacyExists, err := scopedDB.GetValue(ctx, "SELECT to_regclass(?) IS NOT NULL", testCase.legacyTable)
	if err != nil || legacyExists.Bool() {
		t.Fatalf("truncated physical table remains after rename: exists=%v err=%v", legacyExists.Bool(), err)
	}
	evidence, err := scopedDB.GetValue(ctx, "SELECT "+testCase.evidenceColumn+" FROM "+testCase.currentTable+" LIMIT 1")
	if err != nil || evidence.String() != testCase.evidenceValue {
		t.Fatalf("historical evidence changed during rename: value=%q err=%v", evidence.String(), err)
	}
	constraintCount, err := scopedDB.GetValue(ctx, `
SELECT COUNT(1)
  FROM pg_constraint
 WHERE conrelid = to_regclass(?)
   AND conname = ?`, testCase.currentTable, testCase.constraintName)
	if err != nil || constraintCount.Int() != 1 {
		t.Fatalf("historical constraint was not preserved: count=%d err=%v", constraintCount.Int(), err)
	}
}
