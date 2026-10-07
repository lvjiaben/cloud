// This file verifies the compliance foundation migration against isolated PostgreSQL schemas.

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const complianceIntegrationDSNEnv = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

const complianceCodegenDSNEnv = "LINAPRO_IDC_COMPLIANCE_CODEGEN_DSN"

// complianceTriggerContract captures one installed current trigger and its
// deterministic legacy-name counterpart for upgrade fixtures.
type complianceTriggerContract struct {
	currentName string
	legacyName  string
	tableName   string
	definition  string
}

// TestComplianceMigrationIdentifiersFitPostgreSQL prevents PostgreSQL from
// silently truncating migration object names and creating upgrade collisions.
func TestComplianceMigrationIdentifiersFitPostgreSQL(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?im)\bCREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCONSTRAINT\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCREATE(?:\s+UNIQUE)?\s+INDEX\s+IF\s+NOT\s+EXISTS\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCREATE\s+TRIGGER\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCREATE\s+OR\s+REPLACE\s+FUNCTION\s+([a-z0-9_]+)`),
	}
	for _, path := range complianceMigrationPaths(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read compliance migration %q failed: %v", filepath.Base(path), err)
		}
		for _, pattern := range patterns {
			for _, match := range pattern.FindAllSubmatch(content, -1) {
				name := string(match[1])
				if len(name) > 63 {
					t.Errorf("PostgreSQL identifier %q has %d bytes; maximum is 63", name, len(name))
				}
			}
		}
	}
}

// TestComplianceFoundationMigrationPostgreSQL proves the schema, its
// idempotency, plugin ownership, data minimization, and immutable evidence.
func TestComplianceFoundationMigrationPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(complianceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL compliance migration tests", complianceIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create compliance migration admin database failed: %v", err)
	}
	schema := newComplianceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)); err != nil {
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close compliance migration admin database after schema failure: %v", closeErr)
		}
		t.Fatalf("create isolated compliance schema failed: %v", err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "idc_compliance_test_") {
			t.Errorf("refusing to drop unexpected compliance schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated compliance schema failed: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close compliance migration admin database failed: %v", closeErr)
		}
	}()

	scopedDB, err := gdb.New(gdb.ConfigNode{
		Link: appendComplianceDSNParameter(dsn, "search_path="+schema),
	})
	if err != nil {
		t.Fatalf("create scoped compliance database failed: %v", err)
	}
	defer func() {
		if closeErr := scopedDB.Close(ctx); closeErr != nil {
			t.Errorf("close scoped compliance database failed: %v", closeErr)
		}
	}()

	createComplianceDictionaryFixtures(t, ctx, scopedDB)
	executeComplianceMigrations(t, ctx, scopedDB)
	replayComplianceMigrationAfterCurrentSchema(t, ctx, scopedDB, "006-local-supplier-configuration.sql")
	assertComplianceSchemaShape(t, ctx, scopedDB, schema)
	assertComplianceEnterpriseDictionaries(t, ctx, scopedDB)
	assertComplianceSchemaInvariants(t, ctx, scopedDB)
}

// TestComplianceOfficialTriggerDebrandingPostgreSQL proves that the patch
// migration cleans only definition-equivalent legacy triggers and fails closed
// when an old name points at a different trigger contract.
func TestComplianceOfficialTriggerDebrandingPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(complianceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL compliance migration tests", complianceIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create compliance trigger migration admin database failed: %v", err)
	}
	defer adminDB.Close(ctx)
	schema := newComplianceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)); err != nil {
		t.Fatalf("create compliance trigger migration schema failed: %v", err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "idc_compliance_test_") {
			t.Errorf("refusing to drop unexpected compliance trigger schema %q", schema)
			return
		}
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop compliance trigger migration schema failed: %v", dropErr)
		}
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendComplianceDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create compliance trigger migration database failed: %v", err)
	}
	defer database.Close(ctx)
	createComplianceDictionaryFixtures(t, ctx, database)
	executeComplianceMigrations(t, ctx, database)
	contracts := complianceCurrentTriggerContracts(t, ctx, database)
	if len(contracts) != 20 {
		t.Fatalf("current compliance branded-trigger contract count=%d want=20", len(contracts))
	}
	rowCountBefore := complianceBusinessRowCount(t, ctx, database)

	for _, contract := range contracts {
		if _, err = database.Exec(ctx, fmt.Sprintf(
			"ALTER TRIGGER %s ON %s RENAME TO %s",
			quoteCompliancePostgreSQLIdentifier(contract.currentName),
			quoteCompliancePostgreSQLIdentifier(contract.tableName),
			quoteCompliancePostgreSQLIdentifier(contract.legacyName),
		)); err != nil {
			t.Fatalf("prepare old-only compliance trigger %q failed: %v", contract.legacyName, err)
		}
	}
	executeComplianceTriggerDebranding(t, ctx, database)
	assertComplianceTriggerBrandState(t, ctx, database, 20, 0)

	for _, contract := range complianceCurrentTriggerContracts(t, ctx, database) {
		legacyDefinition := strings.Replace(
			contract.definition,
			"TRIGGER "+contract.currentName,
			"TRIGGER "+contract.legacyName,
			1,
		)
		if _, err = database.Exec(ctx, legacyDefinition); err != nil {
			t.Fatalf("prepare new-and-old compliance trigger %q failed: %v", contract.legacyName, err)
		}
	}
	executeComplianceTriggerDebranding(t, ctx, database)
	assertComplianceTriggerBrandState(t, ctx, database, 20, 0)

	first := complianceCurrentTriggerContracts(t, ctx, database)[0]
	if _, err = database.Exec(ctx, fmt.Sprintf(
		"CREATE TRIGGER %s AFTER UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation()",
		quoteCompliancePostgreSQLIdentifier(first.legacyName),
		quoteCompliancePostgreSQLIdentifier(first.tableName),
	)); err != nil {
		t.Fatalf("prepare ambiguous compliance trigger failed: %v", err)
	}
	cleanupSQL := complianceTriggerDebrandingSQL(t)
	if _, err = database.Exec(ctx, cleanupSQL); err == nil || !strings.Contains(err.Error(), "ambiguous legacy idc-compliance trigger") {
		t.Fatalf("ambiguous compliance trigger cleanup error=%v", err)
	}
	if _, err = database.Exec(ctx, fmt.Sprintf(
		"DROP TRIGGER %s ON %s",
		quoteCompliancePostgreSQLIdentifier(first.legacyName),
		quoteCompliancePostgreSQLIdentifier(first.tableName),
	)); err != nil {
		t.Fatalf("remove ambiguous compliance trigger fixture failed: %v", err)
	}
	executeComplianceTriggerDebranding(t, ctx, database)
	executeComplianceTriggerDebranding(t, ctx, database)
	assertComplianceTriggerBrandState(t, ctx, database, 20, 0)
	if rowCountAfter := complianceBusinessRowCount(t, ctx, database); rowCountAfter != rowCountBefore {
		t.Fatalf("compliance business row count changed during trigger cleanup: before=%d after=%d", rowCountBefore, rowCountAfter)
	}
}

// complianceCurrentTriggerContracts reads the authoritative current guards.
func complianceCurrentTriggerContracts(t *testing.T, ctx context.Context, database gdb.DB) []complianceTriggerContract {
	t.Helper()
	rows, err := database.GetAll(ctx, `
SELECT trigger_row.tgname,
       trigger_table.relname AS table_name,
       pg_get_triggerdef(trigger_row.oid, true) AS definition
FROM pg_trigger AS trigger_row
JOIN pg_class AS trigger_table ON trigger_table.oid = trigger_row.tgrelid
JOIN pg_namespace AS trigger_schema ON trigger_schema.oid = trigger_table.relnamespace
WHERE trigger_schema.nspname = current_schema()
  AND NOT trigger_row.tgisinternal
  AND trigger_row.tgname LIKE 'trg_plugin_idc_compliance_%'
ORDER BY trigger_row.tgname`)
	if err != nil {
		t.Fatalf("query current compliance trigger contracts failed: %v", err)
	}
	contracts := make([]complianceTriggerContract, 0, len(rows))
	for _, row := range rows {
		currentName := row["tgname"].String()
		contracts = append(contracts, complianceTriggerContract{
			currentName: currentName,
			legacyName:  strings.Replace(currentName, "trg_plugin_", "trg_plugin_linapro_", 1),
			tableName:   row["table_name"].String(),
			definition:  row["definition"].String(),
		})
	}
	return contracts
}

// complianceTriggerDebrandingSQL reads the patch migration under test.
func complianceTriggerDebrandingSQL(t *testing.T) string {
	t.Helper()
	for _, path := range complianceMigrationPaths(t) {
		if filepath.Base(path) != "026-official-trigger-debranding.sql" {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read compliance trigger debranding migration failed: %v", err)
		}
		return string(content)
	}
	t.Fatal("compliance trigger debranding migration was not found")
	return ""
}

// executeComplianceTriggerDebranding applies the patch migration once.
func executeComplianceTriggerDebranding(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, complianceTriggerDebrandingSQL(t)); err != nil {
		t.Fatalf("execute compliance trigger debranding migration failed: %v", err)
	}
}

// assertComplianceTriggerBrandState verifies current and branded trigger totals.
func assertComplianceTriggerBrandState(t *testing.T, ctx context.Context, database gdb.DB, currentWant, legacyWant int) {
	t.Helper()
	row, err := database.GetOne(ctx, `
SELECT count(*) FILTER (WHERE trigger_row.tgname LIKE 'trg_plugin_idc_compliance_%') AS current_count,
       count(*) FILTER (WHERE trigger_row.tgname LIKE '%linapro%') AS legacy_count
FROM pg_trigger AS trigger_row
JOIN pg_class AS trigger_table ON trigger_table.oid = trigger_row.tgrelid
JOIN pg_namespace AS trigger_schema ON trigger_schema.oid = trigger_table.relnamespace
WHERE trigger_schema.nspname = current_schema()
  AND NOT trigger_row.tgisinternal`)
	if err != nil || row["current_count"].Int() != currentWant || row["legacy_count"].Int() != legacyWant {
		t.Fatalf("compliance trigger state current=%d legacy=%d want current=%d legacy=%d err=%v", row["current_count"].Int(), row["legacy_count"].Int(), currentWant, legacyWant, err)
	}
}

// complianceBusinessRowCount totals plugin-owned rows before and after DDL.
func complianceBusinessRowCount(t *testing.T, ctx context.Context, database gdb.DB) int64 {
	t.Helper()
	tables, err := database.GetAll(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = current_schema()
  AND table_type = 'BASE TABLE'
  AND table_name LIKE 'plugin_idc_compliance_%'
ORDER BY table_name`)
	if err != nil {
		t.Fatalf("list compliance business tables failed: %v", err)
	}
	var total int64
	for _, table := range tables {
		row, countErr := database.GetOne(ctx, "SELECT count(*) AS total FROM "+quoteCompliancePostgreSQLIdentifier(table["table_name"].String()))
		if countErr != nil {
			t.Fatalf("count compliance business table %q failed: %v", table["table_name"].String(), countErr)
		}
		total += row["total"].Int64()
	}
	return total
}

func assertComplianceEnterpriseDictionaries(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	expected := map[string][]string{
		"idc_compliance_enterprise_decision_reason":           {"agent_authorization_invalid", "business_scope_unsupported", "duplicate_registration_risk", "legal_information_mismatch", "manual_review_passed", "materials_incomplete", "materials_unreadable", "registration_identifier_invalid", "registration_status_unverifiable", "representative_identity_mismatch"},
		"idc_compliance_enterprise_applicant_role":            {"authorized_agent", "legal_representative"},
		"idc_compliance_enterprise_encryption_key_status":     {"active", "retired"},
		"idc_compliance_enterprise_material_encryption_state": {"encrypted"},
		"idc_compliance_enterprise_material_status":           {"available", "deleted", "deletion_pending", "legal_hold", "quarantined", "superseded"},
		"idc_compliance_enterprise_material_type":             {"agent_id", "authorization_letter", "business_license", "legal_representative_id", "organization_certificate", "other"},
		"idc_compliance_enterprise_material_upload_status":    {"completed", "reserved"},
		"idc_compliance_enterprise_verification_status":       {"canceled", "draft", "expired", "pending", "rejected", "review_required", "verified", "verifying"},
	}
	for dictType, values := range expected {
		rows, err := database.GetAll(ctx, `
SELECT value
FROM sys_dict_data
WHERE tenant_id = 0 AND dict_type = ?
ORDER BY value`, dictType)
		if err != nil || len(rows) != len(values) {
			t.Fatalf("enterprise dictionary %s row count=%d want=%d err=%v", dictType, len(rows), len(values), err)
		}
		for index, value := range values {
			if actual := rows[index]["value"].String(); actual != value {
				t.Fatalf("enterprise dictionary %s value[%d]=%q want=%q", dictType, index, actual, value)
			}
		}
	}
	root := compliancePluginRoot(t)
	for _, locale := range []string{"en-US", "zh-CN"} {
		path := filepath.Join(root, "manifest", "i18n", locale, "dict.json")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read enterprise dictionary bundle %s: %v", locale, err)
		}
		var bundle struct {
			Dict map[string]map[string]json.RawMessage `json:"dict"`
		}
		if err = json.Unmarshal(content, &bundle); err != nil {
			t.Fatalf("decode enterprise dictionary bundle %s: %v", locale, err)
		}
		for dictType, values := range expected {
			entries := bundle.Dict[dictType]
			if len(entries) == 0 {
				t.Fatalf("enterprise dictionary %s missing from %s", dictType, locale)
			}
			for _, value := range values {
				var item struct {
					Label string `json:"label"`
				}
				if err = json.Unmarshal(entries[value], &item); err != nil || strings.TrimSpace(item.Label) == "" {
					t.Fatalf("enterprise dictionary %s.%s missing label in %s: %v", dictType, value, locale, err)
				}
			}
		}
	}
}

func createComplianceDictionaryFixtures(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL DEFAULT '',
    type VARCHAR(128) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0,
    allow_tenant_override BOOLEAN NOT NULL DEFAULT FALSE,
    remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_compliance_test_dict_type UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    dict_type VARCHAR(128) NOT NULL,
    label VARCHAR(128) NOT NULL DEFAULT '',
    value VARCHAR(128) NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    tag_style VARCHAR(32) NOT NULL DEFAULT 'default',
    status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_compliance_test_dict_data UNIQUE (tenant_id, dict_type, value)
)`); err != nil {
		t.Fatalf("create compliance dictionary fixtures failed: %v", err)
	}
}

// TestComplianceFoundationCodegenPostgreSQL prepares only the explicitly
// named disposable code-generation database and refuses every other target.
func TestComplianceFoundationCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(complianceCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare the compliance DAO code-generation database", complianceCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create compliance code-generation database failed: %v", err)
	}
	defer func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close compliance code-generation database failed: %v", closeErr)
		}
	}()
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "idc_compliance_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	if _, err = database.Exec(ctx, complianceCodegenCleanupSQL); err != nil {
		t.Fatalf("reset disposable compliance code-generation tables failed: %v", err)
	}
	executeComplianceMigrations(t, ctx, database)
}

func assertComplianceSchemaShape(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	schema string,
) {
	t.Helper()
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_compliance_%'`, schema)
	if err != nil || count.Int() != 52 {
		t.Fatalf("compliance table count=%d want=52 err=%v", count.Int(), err)
	}
	softDeletes, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_compliance_%'
  AND column_name = 'deleted_at'`, schema)
	if err != nil || softDeletes.Int() != 0 {
		t.Fatalf("compliance soft-delete column count=%d want=0 err=%v", softDeletes.Int(), err)
	}
	dangerousColumns, err := database.GetAll(ctx, `
SELECT table_name, column_name
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_compliance_%'
  AND column_name IN (
    'raw_payload', 'payload_json', 'face_photo', 'face_video',
    'identity_document_content', 'contract_content', 'invoice_content',
    'access_key_id', 'access_key_secret', 'credential_ciphertext', 'certify_url',
    'registration_token', 'registration_token_digest', 'private_key', 'private_key_ciphertext'
  )`, schema)
	if err != nil || len(dangerousColumns) != 0 {
		t.Fatalf("compliance schema exposes forbidden raw columns: rows=%v err=%v", dangerousColumns, err)
	}
	crossPluginForeignKeys, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_constraint AS constraint_row
JOIN pg_class AS child_table ON child_table.oid = constraint_row.conrelid
JOIN pg_namespace AS child_schema ON child_schema.oid = child_table.relnamespace
JOIN pg_class AS parent_table ON parent_table.oid = constraint_row.confrelid
WHERE constraint_row.contype = 'f'
  AND child_schema.nspname = ?
  AND child_table.relname LIKE 'plugin_idc_compliance_%'
  AND parent_table.relname NOT LIKE 'plugin_idc_compliance_%'`, schema)
	if err != nil || crossPluginForeignKeys.Int() != 0 {
		t.Fatalf(
			"compliance cross-plugin foreign-key count=%d want=0 err=%v",
			crossPluginForeignKeys.Int(),
			err,
		)
	}
	triggerCount, err := database.GetValue(ctx, `
SELECT count(DISTINCT trigger_name)
FROM information_schema.triggers
WHERE trigger_schema = ?
  AND trigger_name IN (
    'trg_plugin_idc_compliance_contract_file_immutable',
    'trg_plugin_idc_compliance_tax_invoice_file_immutable',
    'trg_plugin_idc_compliance_red_alloc_append_only',
    'trg_plugin_idc_compliance_external_evidence_append_only',
    'trg_plugin_idc_compliance_ctplc_append_only',
    'trg_plugin_idc_compliance_ctplfs_append_only',
	    'trg_plugin_idc_compliance_ctplf_append_only',
	    'trg_plugin_idc_compliance_ctplpc_append_only',
	    'trg_plugin_idc_compliance_ctplpr_append_only',
	    'trg_plugin_idc_compliance_ctplpms_append_only',
	    'trg_plugin_idc_compliance_ctplpfm_append_only',
	    'trg_plugin_idc_compliance_ctplprm_append_only',
	    'trg_plugin_idc_compliance_ctplv_guard',
	    'trg_plugin_idc_compliance_contract_snapshot_guard',
	    'trg_idc_compliance_contract_status_evt_append_only',
	    'trg_idc_compliance_contract_replacement_append_only',
	    'trg_plugin_idc_compliance_tax_inv_action_append'
	    ,'trg_plugin_idc_compliance_red_letter_candidate_append'
	    ,'trg_plugin_idc_compliance_red_letter_action_append'
	  )`, schema)
	if err != nil || triggerCount.Int() != 19 {
		t.Fatalf("compliance mutation trigger count=%d want=19 err=%v", triggerCount.Int(), err)
	}
	criticalIndexes, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = ?
  AND indexname IN (
    'uk_plugin_idc_compliance_verification_case_active',
    'idx_plugin_idc_compliance_verification_case_review',
    'idx_plugin_idc_compliance_contract_flow_poll',
    'idx_plugin_idc_compliance_contract_customer',
	    'idx_plugin_idc_compliance_tax_invoice_customer',
	    'idx_plugin_idc_compliance_tax_invoice_action_timeline',
    'idx_plugin_idc_compliance_tax_alloc_billing',
	'uk_plugin_idc_compliance_red_letter_task_refund',
	'idx_plugin_idc_compliance_red_letter_task_customer',
    'idx_plugin_idc_compliance_red_letter_allocation_refund',
	'idx_plugin_idc_compliance_red_letter_candidate_task',
	'idx_plugin_idc_compliance_red_letter_action_task',
    'idx_plugin_idc_compliance_external_evidence_aggregate',
    'uk_plugin_idc_compliance_pva_active',
    'idx_plugin_idc_compliance_pva_recovery',
    'uk_plugin_idc_compliance_ent_key_active',
	'uk_plugin_idc_compliance_verification_ent_request',
	'uk_plugin_idc_compliance_verification_ent_legal_change',
	'uk_plugin_idc_compliance_enterprise_material_request',
	'idx_plugin_idc_compliance_ent_upload_case',
	'idx_plugin_idc_compliance_ent_case_dir',
	'idx_plugin_idc_compliance_ent_status_dir',
	'idx_plugin_idc_compliance_ent_customer_dir',
	'idx_plugin_idc_compliance_ent_customer_current',
	'idx_plugin_idc_compliance_scfg_gate',
	'uk_plugin_idc_compliance_ctplv_enabled',
	'uk_plugin_idc_compliance_ctplv_provider_snapshot',
	'idx_plugin_idc_compliance_ctpl_sync',
	'uk_plugin_idc_compliance_ctpl_creation',
	'idx_plugin_idc_compliance_ctplv_list',
	'idx_plugin_idc_compliance_ctplv_name',
	'idx_plugin_idc_compliance_ctplc_version',
	'idx_plugin_idc_compliance_ctplf_set',
	'idx_plugin_idc_compliance_ctplpc_version',
	'idx_plugin_idc_compliance_ctplpr_version',
	'uk_plugin_idc_compliance_contract_creation',
	'idx_plugin_idc_compliance_contract_order_history',
	'uk_plugin_idc_compliance_contract_replacement_no',
	'uk_plugin_idc_compliance_contract_replacement_old',
	'uk_plugin_idc_compliance_contract_replacement_new',
	'uk_idc_compliance_contract_replacement_idem',
	'idx_plugin_idc_compliance_contract_replacement_chain',
	'idx_plugin_idc_compliance_contract_fulfillment_gate',
	'uk_idc_compliance_contract_status_evt_route_version',
	'uk_plugin_idc_compliance_contract_status_event_delivery',
	'idx_idc_compliance_contract_status_evt_contract',
	'idx_plugin_idc_compliance_contract_status_event_flow'
  )`, schema)
	if err != nil || criticalIndexes.Int() != 47 {
		t.Fatalf("compliance critical index count=%d want=47 err=%v", criticalIndexes.Int(), err)
	}
}

func assertComplianceSchemaInvariants(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	supplierReadyConstraint, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_constraint
WHERE conname = 'ck_plugin_idc_compliance_scfg_credential_ready'
  AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass`)
	if err != nil || supplierReadyConstraint.Int() != 1 {
		t.Fatalf("supplier credential-ready constraint count=%d want=1 err=%v", supplierReadyConstraint.Int(), err)
	}
	supplierProofConstraints, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_constraint
WHERE conname IN (
    'ck_plugin_idc_compliance_scfg_subject_proof',
    'ck_plugin_idc_compliance_scfg_subject_pending_proof'
)
  AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass`)
	if err != nil || supplierProofConstraints.Int() != 2 {
		t.Fatalf("supplier proof constraint count=%d want=2 err=%v", supplierProofConstraints.Int(), err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_encryption_key
    (key_no, secret_ref, generation, status_code, version)
VALUES
    ('enterprise_material_key_v1', 'SEC_0123456789abcdef0123456789abcdef', 1, 'active', 1)`); err != nil {
		t.Fatalf("insert enterprise encryption key reference failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_verification_case
    (case_no, customer_no, principal_no, subject_type_code, provider_code, consent_no,
     request_idempotency_key_digest, request_fingerprint, encryption_key_no)
VALUES
    ('kyc-personal-1', 'customer-1', 'principal-1', 'personal', 'aliyun_face', 'consent-1',
     NULL, NULL, NULL),
    ('kyc-enterprise-1', 'customer-2', 'principal-2', 'enterprise', 'manual_enterprise', 'consent-2',
     repeat('1', 64), repeat('2', 64), 'enterprise_material_key_v1'),
    ('kyc-enterprise-2', 'customer-3', 'principal-3', 'enterprise', 'manual_enterprise', 'consent-3',
     repeat('3', 64), repeat('4', 64), 'enterprise_material_key_v1')`); err != nil {
		t.Fatalf("insert compliance verification cases failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_verification_case
    (case_no, customer_no, principal_no, subject_type_code, provider_code, consent_no)
VALUES ('kyc-personal-duplicate', 'customer-1', 'principal-1', 'personal', 'aliyun_face', 'consent-3')`); err == nil {
		t.Fatal("duplicate active verification case was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_verification_case
    (tenant_id, case_no, customer_no, principal_no, subject_type_code, provider_code, consent_no)
VALUES (1, 'kyc-wrong-tenant', 'customer-4', 'principal-4', 'personal', 'aliyun_face', 'consent-4')`); err == nil {
		t.Fatal("non-platform compliance tenant was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_provider_configuration
    (configuration_no, provider_code, status_code, scene_id,
     credential_generation)
VALUES
	('aliyun-config-incomplete', 'aliyun_face', 'configured', '100000001', 1)`); err == nil {
		t.Fatal("configured Aliyun provider accepted a missing credential envelope")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_supplier_configuration
    (configuration_no, provider_code, credential_status_code,
     credential_generation, capability_evidence_code, adapter_enabled)
VALUES
    ('local-supplier-tencent_essbasic', 'tencent_essbasic', 'configured', 1,
     'external_confirmation_required', FALSE)`); err == nil {
		t.Fatal("configured local supplier accepted a missing secretcap reference")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_supplier_configuration
    (configuration_no, provider_code, credential_status_code,
     capability_evidence_code, adapter_enabled)
VALUES
	('local-supplier-tencent-invalid-evidence', 'tencent_essbasic', 'unconfigured',
	 'external_confirmation_required', TRUE)`); err == nil {
		t.Fatal("Tencent adapter bypassed the official public contract gate")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_supplier_configuration
    (configuration_no, provider_code, credential_status_code, subject_status_code,
     subject_binding_no, capability_evidence_code, adapter_enabled)
VALUES
	('local-supplier-tencent-pending', 'tencent_essbasic', 'unconfigured', 'binding',
	 NULL, 'official_public_contract', TRUE)`); err == nil {
		t.Fatal("Tencent adapter activated before the local subject became active")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_supplier_configuration
    (configuration_no, provider_code, credential_status_code, subject_status_code,
     subject_binding_no, capability_evidence_code, adapter_enabled)
VALUES
	('local-supplier-tencent-active', 'tencent_essbasic', 'unconfigured', 'active',
	 'lsb_0123456789abcdef0123456789abcdef', 'official_public_contract', TRUE)`); err != nil {
		t.Fatalf("active Tencent onboarding result was rejected: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_supplier_configuration
    (configuration_no, provider_code, credential_status_code,
     capability_evidence_code, adapter_enabled)
VALUES
    ('local-supplier-nuonuo_invoice', 'nuonuo_invoice', 'unconfigured',
     'external_confirmation_required', TRUE)`); err == nil {
		t.Fatal("local supplier adapter bypassed the external evidence gate")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_supplier_configuration
    (configuration_no, provider_code, credential_status_code,
     capability_evidence_code, adapter_enabled)
VALUES
	('local-supplier-nuonuo-official', 'nuonuo_invoice', 'unconfigured',
	 'official_public_contract', FALSE)`); err == nil {
		t.Fatal("NuoNuo accepted Tencent-only public-contract evidence")
	}
	legacySubjectColumns, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'plugin_idc_compliance_supplier_configuration'
  AND column_name = 'merchant_subject_no'`)
	if err != nil || legacySubjectColumns.Int() != 0 {
		t.Fatalf("legacy merchant subject column count=%d want=0 err=%v", legacySubjectColumns.Int(), err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_provider_configuration
    (configuration_no, provider_code, status_code, scene_id,
     credential_generation)
VALUES
    ('aliyun-config-unrecoverable', 'aliyun_face', 'updating', '100000001', 0)`); err == nil {
		t.Fatal("updating Aliyun provider accepted a missing idempotent recovery marker")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_provider_configuration
    (configuration_no, provider_code, status_code, scene_id,
     credential_generation, last_mutation_digest)
VALUES
    ('aliyun-config-missing-operation', 'aliyun_face', 'updating', '100000001', 0,
     'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')`); err == nil {
		t.Fatal("updating Aliyun provider accepted a missing internal recovery operation key")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_provider_configuration
    (configuration_no, provider_code, status_code, scene_id,
     credential_secret_ref, credential_generation)
VALUES
    ('aliyun-config-1', 'aliyun_face', 'configured', '100000001',
	 'secret-access-key-credential', 1)`); err != nil {
		t.Fatalf("insert secret-reference-only Aliyun configuration failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_provider_configuration
SET last_validation_code = 'raw_provider_message'
WHERE configuration_no = 'aliyun-config-1'`); err == nil {
		t.Fatal("provider configuration accepted an unbounded validation status")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_personal_verification_attempt
    (attempt_no, case_no, configuration_no, provider_code, configuration_version,
     outer_order_no)
VALUES
    ('attempt-1', 'kyc-personal-1', 'aliyun-config-1', 'aliyun_face', 1,
     '0123456789abcdef0123456789abcdef')`); err != nil {
		t.Fatalf("insert personal verification attempt failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_personal_verification_attempt
    (attempt_no, case_no, configuration_no, provider_code, configuration_version,
     outer_order_no)
VALUES
    ('attempt-enterprise', 'kyc-enterprise-1', 'aliyun-config-1', 'aliyun_face', 1,
     '1123456789abcdef0123456789abcdef')`); err == nil {
		t.Fatal("personal verification attempt was attached to an enterprise case")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_personal_verification_attempt
    (attempt_no, case_no, configuration_no, provider_code, configuration_version,
     outer_order_no)
VALUES
    ('attempt-duplicate-active', 'kyc-personal-1', 'aliyun-config-1', 'aliyun_face', 1,
     '2123456789abcdef0123456789abcdef')`); err == nil {
		t.Fatal("duplicate active personal verification attempt was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_material_upload
    (material_no, case_no, material_type_code, idempotency_key_digest,
     request_fingerprint, status_code)
VALUES
    ('material-1', 'kyc-enterprise-1', 'business_license', repeat('5', 64), repeat('6', 64), 'completed'),
    ('material-bad-digest', 'kyc-enterprise-1', 'other', repeat('7', 64), repeat('8', 64), 'completed'),
    ('material-other-case', 'kyc-enterprise-2', 'other', repeat('b', 64), repeat('c', 64), 'completed')`); err != nil {
		t.Fatalf("insert enterprise material upload reservations failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_material
    (material_no, case_no, material_type_code, private_object_ref, content_sha256, media_type,
     byte_size, encryption_key_no, stored_byte_size, idempotency_key_digest, request_fingerprint)
VALUES
    ('material-no-reservation', 'kyc-enterprise-1', 'other', 'private://enterprise/no-reservation',
     repeat('0', 64), 'application/pdf', 32, 'enterprise_material_key_v1', 64,
     repeat('d', 64), repeat('e', 64))`); err == nil {
		t.Fatal("enterprise material without an upload reservation was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_material
    (material_no, case_no, material_type_code, private_object_ref, content_sha256, media_type,
     byte_size, encryption_key_no, stored_byte_size, idempotency_key_digest, request_fingerprint)
VALUES
    ('material-1', 'kyc-enterprise-1', 'business_license', 'private://enterprise/material-1',
     repeat('a', 64), 'application/pdf', 1024, 'enterprise_material_key_v1', 1056,
     repeat('5', 64), repeat('6', 64))`); err != nil {
		t.Fatalf("insert encrypted enterprise material metadata failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_material
    (material_no, case_no, material_type_code, private_object_ref, content_sha256, media_type,
     byte_size, encryption_key_no, stored_byte_size, idempotency_key_digest, request_fingerprint)
VALUES
    ('material-bad-digest', 'kyc-enterprise-1', 'other', 'private://enterprise/bad',
     'plaintext', 'application/pdf', 32, 'enterprise_material_key_v1', 64,
     repeat('7', 64), repeat('8', 64))`); err == nil {
		t.Fatal("enterprise material without a SHA-256 digest was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_material
    (material_no, case_no, material_type_code, private_object_ref, content_sha256, media_type,
     byte_size, encryption_key_no, stored_byte_size, idempotency_key_digest, request_fingerprint)
VALUES
    ('material-personal-case', 'kyc-personal-1', 'other', 'private://enterprise/personal-case',
     repeat('0', 64), 'application/pdf', 32, 'enterprise_material_key_v1', 64,
     repeat('9', 64), repeat('a', 64))`); err == nil {
		t.Fatal("enterprise material was attached to a personal verification case")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_enterprise_material
    (material_no, case_no, material_type_code, private_object_ref, content_sha256, media_type,
     byte_size, encryption_key_no, stored_byte_size, idempotency_key_digest, request_fingerprint)
VALUES
    ('material-other-case', 'kyc-enterprise-2', 'other', 'private://enterprise/other-case',
     repeat('0', 64), 'application/pdf', 32, 'enterprise_material_key_v1', 64,
     repeat('b', 64), repeat('c', 64));
UPDATE plugin_idc_compliance_enterprise_material
SET status_code = 'superseded', superseded_by_material_no = 'material-other-case'
WHERE material_no = 'material-1'`); err == nil {
		t.Fatal("enterprise material was superseded across verification cases")
	}

	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_template
    (template_no, origin_code)
VALUES
    ('template-1', 'local');
INSERT INTO plugin_idc_compliance_contract_template_version
    (template_no, version_no, name, scope_code, signing_order_code, status_code)
VALUES
    ('template-1', 1, 'Template 1', 'all_customers', 'customer_first', 'draft');
INSERT INTO plugin_idc_compliance_contract
    (contract_no, customer_no, principal_no, order_no, contract_type_code,
     template_no, template_version, variable_snapshot_ref, variable_snapshot_sha256,
     signer_snapshot_sha256, currency_code, amount_minor, subject_type_code,
     signing_order_code, snapshot_key_ref, order_snapshot_sha256)
VALUES
    ('contract-1', 'customer-1', 'principal-1', 'order-1', 'service_purchase',
     'template-1', 1, 'private://contract/snapshot-1', repeat('b', 64), repeat('c', 64),
     'CNY', 10000, 'individual', 'customer_first',
     'private://contract/snapshot-1', repeat('d', 64));
INSERT INTO plugin_idc_compliance_contract_flow
    (flow_no, contract_no, sequence_no, purpose_code, provider_code, idempotency_sha256,
     request_snapshot_ref, request_snapshot_sha256, status_code, completed_at)
VALUES
    ('flow-1', 'contract-1', 1, 'issue', 'tencent_ess', repeat('d', 64),
     'private://contract/request-1', repeat('e', 64), 'completed', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_compliance_contract_signer
    (signer_no, flow_no, sequence_no, subject_type_code, subject_no, role_code,
     snapshot_sha256, status_code, signed_at)
VALUES
    ('signer-1', 'flow-1', 1, 'personal', 'customer-1', 'customer',
     repeat('f', 64), 'signed', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_compliance_contract_file
    (file_no, contract_no, flow_no, file_type_code, private_object_ref, content_sha256,
     media_type, byte_size, status_code, immutable, archived_at, supplier_observed_at)
VALUES
    ('contract-file-1', 'contract-1', 'flow-1', 'final_pdf', 'private://contract/final-1',
     repeat('1', 64), 'application/pdf', 2048, 'archived', TRUE,
     CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert contract aggregate and archived file failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (contract_no, customer_no, principal_no, order_no, contract_type_code,
     template_no, template_version, variable_snapshot_ref, variable_snapshot_sha256,
     signer_snapshot_sha256, currency_code, amount_minor, subject_type_code,
     signing_order_code, snapshot_key_ref, order_snapshot_sha256)
VALUES
    ('contract-duplicate-order', 'customer-1', 'principal-1', 'order-1', 'service_purchase',
     'template-1', 1, 'private://contract/duplicate', repeat('2', 64), repeat('3', 64),
     'CNY', 10000, 'individual', 'customer_first',
	 'private://contract/duplicate', repeat('4', 64))`); err != nil {
		t.Fatalf("insert a second immutable contract version for one order: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_replacement
    (replacement_no, old_contract_no, new_contract_no, idempotency_sha256)
VALUES ('replacement-1', 'contract-1', 'contract-duplicate-order', repeat('5', 64))`); err != nil {
		t.Fatalf("insert immutable contract replacement relation: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_replacement
SET new_contract_no = 'contract-1'
WHERE replacement_no = 'replacement-1'`); err == nil {
		t.Fatal("contract replacement history was mutable")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_contract_replacement
WHERE replacement_no = 'replacement-1'`); err == nil {
		t.Fatal("contract replacement history was deletable")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (contract_no, customer_no, principal_no, order_no, contract_type_code,
     template_no, template_version, variable_snapshot_ref, variable_snapshot_sha256,
     signer_snapshot_sha256, currency_code, amount_minor, subject_type_code,
     signing_order_code, snapshot_key_ref, order_snapshot_sha256,
     created_at, signing_expires_at)
VALUES
    ('contract-invalid-expiry', 'customer-1', 'principal-1', 'order-invalid-expiry',
     'service_purchase', 'template-1', 1, 'private://contract/invalid-expiry',
     repeat('2', 64), repeat('3', 64), 'CNY', 10000, 'individual', 'customer_first',
     'private://contract/invalid-expiry', repeat('4', 64),
     CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '8 days')`); err == nil {
		t.Fatal("a contract with a non-seven-day signing deadline was accepted")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_file
SET private_object_ref = 'private://contract/tampered'
WHERE file_no = 'contract-file-1'`); err == nil {
		t.Fatal("archived contract file metadata was mutable")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_file
SET status_code = 'legal_hold', legal_hold = TRUE,
    retention_until = CURRENT_TIMESTAMP + INTERVAL '10 years'
WHERE file_no = 'contract-file-1'`); err != nil {
		t.Fatalf("archived contract file rejected legal-hold governance metadata: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_file
SET content_sha256 = repeat('0', 64)
WHERE file_no = 'contract-file-1'`); err == nil {
		t.Fatal("legal-hold contract file content was mutable")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_contract_file
WHERE file_no = 'contract-file-1'`); err == nil {
		t.Fatal("contract file metadata was deletable")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (contract_no, customer_no, principal_no, order_no, contract_type_code,
     template_no, template_version, variable_snapshot_ref, variable_snapshot_sha256,
     signer_snapshot_sha256, currency_code, amount_minor, subject_type_code,
     signing_order_code, snapshot_key_ref, order_snapshot_sha256)
VALUES
    ('contract-2', 'customer-1', 'principal-1', 'order-2', 'service_purchase',
     'template-1', 1, 'private://contract/snapshot-2', repeat('0', 64), repeat('1', 64),
     'CNY', 10000, 'individual', 'customer_first',
     'private://contract/snapshot-2', repeat('2', 64));
INSERT INTO plugin_idc_compliance_contract_file
    (file_no, contract_no, flow_no, file_type_code, private_object_ref, content_sha256,
     media_type, byte_size)
VALUES
    ('contract-file-cross-flow', 'contract-2', 'flow-1', 'preview',
     'private://contract/cross-flow', repeat('2', 64), 'application/pdf', 128)`); err == nil {
		t.Fatal("contract file referenced a flow owned by another contract")
	}

	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice
    (tax_invoice_no, request_sha256, customer_no, principal_no,
     seller_snapshot_ref, seller_snapshot_sha256, buyer_snapshot_ref, buyer_snapshot_sha256,
     invoice_type_code, status_code, tax_currency_code, gross_amount_minor,
     net_amount_minor, tax_amount_minor, billing_currency_code, issued_at)
VALUES
    ('tax-invoice-1', repeat('2', 64), 'customer-1', 'principal-1',
     'private://invoice/seller-1', repeat('3', 64), 'private://invoice/buyer-1', repeat('4', 64),
     'vat_electronic_normal', 'issued', 'CNY', 1000, 900, 100, 'CNY', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_compliance_tax_invoice_allocation
    (allocation_no, tax_invoice_no, billing_invoice_no, order_no,
     allocated_gross_minor, allocated_net_minor, allocated_tax_minor)
VALUES ('allocation-1', 'tax-invoice-1', 'billing-invoice-opaque-1', 'order-1', 1000, 900, 100);
INSERT INTO plugin_idc_compliance_tax_invoice_item
    (item_no, tax_invoice_no, allocation_no, billing_invoice_item_no,
     line_no, item_snapshot_ref, item_snapshot_sha256,
     tax_classification_code, tax_rate_ppm, gross_amount_minor, net_amount_minor, tax_amount_minor)
VALUES
    ('invoice-item-1', 'tax-invoice-1', 'allocation-1', 'billing-invoice-item-opaque-1',
     1, 'private://invoice/item-1', repeat('5', 64),
     'server_service', 111111, 1000, 900, 100);
INSERT INTO plugin_idc_compliance_tax_invoice_file
    (file_no, tax_invoice_no, file_type_code, private_object_ref, content_sha256,
     media_type, byte_size, status_code, immutable, archived_at)
VALUES
    ('invoice-file-1', 'tax-invoice-1', 'invoice_pdf', 'private://invoice/pdf-1',
     repeat('6', 64), 'application/pdf', 1024, 'archived', TRUE, CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_compliance_red_letter_task
    (task_no, tax_invoice_no, request_sha256, refund_operation_no, reason_code,
     gross_amount_minor, net_amount_minor, tax_amount_minor, customer_no, currency_code)
VALUES
    ('red-letter-1', 'tax-invoice-1', repeat('7', 64), 'refund-operation-opaque-1',
     'customer_refund', 1000, 900, 100, 'customer-1', 'CNY');
INSERT INTO plugin_idc_compliance_red_letter_allocation
    (allocation_no, task_no, tax_invoice_no, tax_invoice_allocation_no, tax_invoice_item_no, refund_allocation_no,
     gross_amount_minor, net_amount_minor, tax_amount_minor)
VALUES
    ('red-allocation-1', 'red-letter-1', 'tax-invoice-1', 'allocation-1', 'invoice-item-1', 'refund-allocation-opaque-1',
     1000, 900, 100)`); err != nil {
		t.Fatalf("insert tax invoice, allocation, item, file, and red-letter task failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_red_letter_allocation
SET gross_amount_minor = 999
WHERE allocation_no = 'red-allocation-1'`); err == nil {
		t.Fatal("red-letter allocation history was mutable")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice
    (tax_invoice_no, request_sha256, customer_no, principal_no,
     seller_snapshot_ref, seller_snapshot_sha256, buyer_snapshot_ref, buyer_snapshot_sha256,
     invoice_type_code, status_code, tax_currency_code, gross_amount_minor,
     net_amount_minor, tax_amount_minor, billing_currency_code, issued_at)
VALUES
    ('tax-invoice-2', repeat('d', 64), 'customer-1', 'principal-1',
     'private://invoice/seller-2', repeat('e', 64), 'private://invoice/buyer-2', repeat('f', 64),
     'vat_electronic_normal', 'issued', 'CNY', 1000, 900, 100, 'CNY', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_compliance_tax_invoice_allocation
    (allocation_no, tax_invoice_no, billing_invoice_no, order_no,
     allocated_gross_minor, allocated_net_minor, allocated_tax_minor)
VALUES ('allocation-2', 'tax-invoice-2', 'billing-invoice-opaque-2', 'order-2', 1000, 900, 100);
INSERT INTO plugin_idc_compliance_red_letter_allocation
    (allocation_no, task_no, tax_invoice_no, tax_invoice_allocation_no, tax_invoice_item_no, refund_allocation_no,
     gross_amount_minor, net_amount_minor, tax_amount_minor)
VALUES
    ('red-allocation-cross-invoice', 'red-letter-1', 'tax-invoice-1', 'allocation-2', 'invoice-item-1',
     'refund-allocation-opaque-2', 1000, 900, 100)`); err == nil {
		t.Fatal("red-letter allocation referenced another tax invoice allocation")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_tax_invoice_file
SET content_sha256 = repeat('8', 64)
WHERE file_no = 'invoice-file-1'`); err == nil {
		t.Fatal("archived tax-invoice file metadata was mutable")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice_action
    (action_no, tax_invoice_no, action_code, idempotency_digest, request_fingerprint,
     actor_user_id, before_status_code, after_status_code, before_version, after_version,
     invoice_type_code, provider_code)
VALUES
    ('tax-invoice-action-1', 'tax-invoice-1', 'approve', repeat('a', 64), repeat('b', 64),
     1, 'pending_review', 'approved_pending_submission', 1, 2,
     'vat_electronic_normal', 'nuonuo_invoice')`); err != nil {
		t.Fatalf("insert append-only tax-invoice action failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_tax_invoice_action
SET provider_code = 'changed'
WHERE action_no = 'tax-invoice-action-1'`); err == nil {
		t.Fatal("tax-invoice action history was mutable")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_tax_invoice_action
WHERE action_no = 'tax-invoice-action-1'`); err == nil {
		t.Fatal("tax-invoice action history was deletable")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_tax_invoice_file
SET status_code = 'deletion_pending', retention_until = CURRENT_TIMESTAMP
WHERE file_no = 'invoice-file-1'`); err != nil {
		t.Fatalf("archived tax-invoice file rejected retention governance metadata: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice
    (tax_invoice_no, request_sha256, customer_no, principal_no,
     seller_snapshot_ref, seller_snapshot_sha256, buyer_snapshot_ref, buyer_snapshot_sha256,
     invoice_type_code, tax_currency_code, gross_amount_minor, net_amount_minor,
     tax_amount_minor, billing_currency_code)
VALUES
    ('tax-invoice-invalid-currency', repeat('9', 64), 'customer-1', 'principal-1',
     'private://invoice/seller-2', repeat('a', 64), 'private://invoice/buyer-2', repeat('b', 64),
     'vat_electronic_normal', 'cny', 100, 90, 10, 'cny')`); err == nil {
		t.Fatal("lowercase tax-invoice currency was accepted")
	}

	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_external_evidence
    (evidence_no, aggregate_type_code, aggregate_no, source_system_code,
     source_event_ref, evidence_type_code, payload_sha256, occurred_at)
VALUES
    ('evidence-1', 'contract', 'contract-1', 'tencent_ess',
     'event-1', 'flow_completed', repeat('c', 64), CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert append-only external evidence failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_external_evidence
SET disposition_code = 'quarantined'
WHERE evidence_no = 'evidence-1'`); err == nil {
		t.Fatal("external evidence was mutable")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_external_evidence
WHERE evidence_no = 'evidence-1'`); err == nil {
		t.Fatal("external evidence was deletable")
	}
	assertContractTemplateInvariants(t, ctx, database)
}

func assertContractTemplateInvariants(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_template
    (template_no, origin_code, provider_code, provider_template_ref,
     provider_state_code, provider_last_seen_sync_no, provider_updated_at)
VALUES
    ('ctpl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'tencent_synced', 'tencent_essbasic',
     'provider-template-a', 'active', 'cts_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_compliance_contract_template_version
    (template_no, version_no, name, scope_code, signing_order_code, status_code,
     provider_snapshot_digest, lock_version)
VALUES
    ('ctpl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1, 'Provider template A', 'enterprise',
     'customer_first', 'enabled', repeat('a', 64), 1)`); err != nil {
		t.Fatalf("insert enabled synchronized template failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_template_version
    (template_no, version_no, name, scope_code, signing_order_code, status_code,
     provider_snapshot_digest, lock_version)
VALUES
    ('ctpl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 2, 'Provider template A v2', 'enterprise',
     'customer_first', 'enabled', repeat('b', 64), 1)`); err == nil {
		t.Fatal("a second enabled version of one logical template was accepted")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_template_version
SET name = 'tampered', lock_version = 2
WHERE template_no = 'ctpl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' AND version_no = 1`); err == nil {
		t.Fatal("enabled template metadata was mutable")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_template
    (template_no, origin_code)
VALUES ('ctpl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'local');
INSERT INTO plugin_idc_compliance_contract_template_version
    (template_no, version_no, name, scope_code, signing_order_code, status_code)
VALUES
    ('ctpl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 1, 'Local template B', 'enterprise',
     'parallel', 'draft');
INSERT INTO plugin_idc_compliance_contract_template_content
    (content_revision_no, template_no, version_no, revision_no,
     command_idempotency_digest, command_fingerprint, source_mode_code,
     encryption_key_no, source_object_ref, source_sha256, source_media_type,
     source_byte_size, conversion_status_code, created_by_user_id)
VALUES
    ('ctc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
     'ctpl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 1, 1,
     repeat('0', 64), repeat('1', 64), 'uploaded_file', 'secret-version-1',
     'private://contract-template/source-a', repeat('c', 64), 'application/pdf', 100,
     'source_saved', 1);
INSERT INTO plugin_idc_compliance_contract_template_content
    (content_revision_no, template_no, version_no, revision_no,
     command_idempotency_digest, command_fingerprint, source_mode_code,
     encryption_key_no, source_object_ref, source_sha256, source_media_type,
     source_byte_size, pdf_object_ref, pdf_sha256, pdf_byte_size, pdf_page_count,
     conversion_status_code, created_by_user_id)
VALUES
    ('ctc_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
     'ctpl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 1, 2,
     repeat('2', 64), repeat('3', 64), 'online_editor', 'secret-version-1',
     'private://contract-template/source-b', repeat('d', 64), 'text/html', 120,
     'private://contract-template/pdf-b', repeat('e', 64), 256, 1, 'ready', 1);
INSERT INTO plugin_idc_compliance_contract_template_field_set
    (field_set_no, template_no, version_no, content_revision_no,
     command_idempotency_digest, command_fingerprint, created_by_user_id)
VALUES
    ('ctfs_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
     'ctpl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 1,
     'ctc_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', repeat('4', 64), repeat('5', 64), 1);
INSERT INTO plugin_idc_compliance_contract_template_field
    (field_no, field_set_no, template_no, version_no, sequence_no, field_type_code,
     party_code, page_no, position_x_ppm, position_y_ppm, width_ppm, height_ppm)
VALUES
    ('ctf_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
     'ctfs_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
     'ctpl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 1, 1, 'signature', 'customer',
     1, 10000, 10000, 200000, 100000)`); err != nil {
		t.Fatalf("insert append-only template authoring history failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_template_content
SET source_object_ref = 'private://contract-template/tampered'
WHERE content_revision_no = 'ctc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`); err == nil {
		t.Fatal("template content revision was mutable")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_contract_template_field
WHERE field_no = 'ctf_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`); err == nil {
		t.Fatal("template field placement history was deletable")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_contract_template_version
WHERE template_no = 'template-1' AND version_no = 1`); err == nil {
		t.Fatal("a template version already used by a contract was deletable")
	}
}

func complianceMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compliance migration test path failed")
	}
	paths, err := filepath.Glob(filepath.Join(
		filepath.Dir(file), "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql",
	))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve compliance migration paths failed: %v", err)
	}
	return paths
}

func compliancePluginRoot(t *testing.T) string {
	t.Helper()
	paths := complianceMigrationPaths(t)
	return filepath.Dir(filepath.Dir(filepath.Dir(paths[0])))
}

func executeComplianceMigrations(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	paths := complianceMigrationPaths(t)
	for _, path := range paths {
		// Re-run each migration immediately after it is first applied. Replaying
		// the complete historical chain after later migrations have deliberately
		// removed legacy columns does not model the versioned installer.
		for attempt := 1; attempt <= 2; attempt++ {
			migration, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read compliance migration %q failed: %v", filepath.Base(path), err)
			}
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf(
					"execute compliance migration %q attempt %d failed: %v",
					filepath.Base(path),
					attempt,
					err,
				)
			}
		}
	}
}

func replayComplianceMigrationAfterCurrentSchema(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	fileName string,
) {
	t.Helper()
	for _, path := range complianceMigrationPaths(t) {
		if filepath.Base(path) != fileName {
			continue
		}
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read compliance migration %q for current-schema replay: %v", fileName, err)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("replay compliance migration %q against current schema failed: %v", fileName, err)
		}
		return
	}
	t.Fatalf("compliance migration %q was not found", fileName)
}

func newComplianceIntegrationSchema(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate compliance integration schema failed: %v", err)
	}
	return "idc_compliance_test_" + hex.EncodeToString(content[:])
}

func appendComplianceDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func quoteCompliancePostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

const complianceCodegenCleanupSQL = `
DROP TABLE IF EXISTS plugin_idc_compliance_contract_archive_job CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_status_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_replacement CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_auto_sign_usage CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_auto_sign_authorization_template CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_auto_sign_authorization CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_auto_sign_auth_template CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_auto_sign_auth CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_signer_identity_attempt CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_signer_profile CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_template_field CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_template_field_set CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_template_content CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_template_version CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_template CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_gateway_enrollment_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_gateway_enrollment CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_supplier_configuration CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_personal_verification_attempt CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_provider_configuration CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_external_evidence CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_red_letter_intake_cursor CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_red_letter_action CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_red_letter_candidate CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_red_letter_allocation CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_red_letter_task CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_tax_invoice_file CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_tax_invoice_action CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_tax_invoice_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_tax_invoice_allocation CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_tax_invoice CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_file CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_signer CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract_flow CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_contract CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_enterprise_material CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_enterprise_material_upload CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_verification_case CASCADE;
DROP TABLE IF EXISTS plugin_idc_compliance_enterprise_encryption_key CASCADE;
DROP FUNCTION IF EXISTS plugin_idc_compliance_reject_append_only_mutation() CASCADE;
DROP FUNCTION IF EXISTS plugin_idc_compliance_reject_archived_file_mutation() CASCADE;
DROP FUNCTION IF EXISTS plugin_idc_compliance_reject_template_history_mutation() CASCADE;
DROP FUNCTION IF EXISTS plugin_idc_compliance_guard_template_version() CASCADE;
DROP FUNCTION IF EXISTS plugin_idc_compliance_reject_gateway_enrollment_delete() CASCADE;`
