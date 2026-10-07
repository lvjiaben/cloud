// This file verifies the installation-local migration boundary.
package migration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const integrationDSNEnv = "LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"

func TestEmbeddedLocalMigrationContract(t *testing.T) {
	migrations, err := load()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) != 21 {
		t.Fatalf("migration count=%d want=21", len(migrations))
	}
	var contents strings.Builder
	for _, item := range migrations {
		if len(item.Checksum) != 64 {
			t.Fatalf("migration %s has invalid checksum %q", item.Name, item.Checksum)
		}
		contents.WriteString(item.SQL)
	}
	content := contents.String()
	for _, required := range []string{
		"DROP TABLE IF EXISTS idc_gateway_registration_token",
		"DROP TABLE IF EXISTS idc_gateway_instance",
		"DROP TABLE IF EXISTS idc_gateway_instance_delivery",
		"idc_gateway_local_business_request",
		"idc_gateway_process_nonce",
		`"request_no" VARCHAR(40),`,
		"idc_gateway_supplier_operation",
		"idc_gateway_supplier_evidence",
		"idc_gateway_supplier_callback",
		"idc_gateway_supplier_circuit",
		"idc_gateway_supplier_takeover",
		"employee_principal_no",
		"local gateway immutable history cannot be changed",
		"DROP TABLE IF EXISTS idc_gateway_settlement_account",
		"DROP TABLE IF EXISTS idc_gateway_price_version",
		"idc_gateway_supplier_plan_snapshot",
		"idc_gateway_supplier_usage_snapshot",
		"idc_gateway_supplier_budget_policy",
		"idc_gateway_supplier_reconciliation",
		"idc_gateway_supplier_sla_snapshot",
		"idc_gateway_supplier_collection_state",
		"idc_gateway_data_lifecycle_policy",
		"idc_gateway_legal_hold_event",
		"idc_gateway_evidence_export",
		"idc_gateway_supplier_authorization_event",
		"idc_gateway_restore_verification",
		"idc_gateway_supplier_processing_state",
		"idc_gateway_retention_disposition",
		"idc_gateway_local_supplier_instance",
		"idc_gateway_local_supplier_subject",
		"idc_gateway_local_subject_lookup",
		"idc_gateway_local_identity_operation",
		"idc_gateway_local_rate_limit_window",
		"idc_gateway_local_supplier_write_attempt",
		"idc_gateway_supplier_health_sample",
		"idc_gateway_local_supplier_route",
		"idc_gateway_local_callback_evidence",
		"idc_gateway_local_route_delivery",
		"idc_gateway_local_supplier_takeover",
		"idc_gateway_local_takeover_evidence",
		"idc_gateway_tencent_contract_report_job",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("migration set is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"token_plain", "private_key", "secret_id", "secret_key", "subject_plaintext",
		"double precision", " float", " real",
	} {
		if strings.Contains(strings.ToLower(content), forbidden) {
			t.Fatalf("migration contains forbidden secret or numeric field %q", forbidden)
		}
	}
}

func TestPostgreSQLIdentifiersFitLimit(t *testing.T) {
	migrations, err := load()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?im)\bCONSTRAINT\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCREATE(?:\s+UNIQUE)?\s+INDEX\s+IF\s+NOT\s+EXISTS\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+([a-z0-9_]+)`),
		regexp.MustCompile(`(?im)\bCREATE\s+TRIGGER\s+([a-z0-9_]+)`),
	}
	for _, item := range migrations {
		for _, pattern := range patterns {
			for _, match := range pattern.FindAllStringSubmatch(item.SQL, -1) {
				if len(match[1]) > 63 {
					t.Errorf("PostgreSQL identifier %q has %d bytes", match[1], len(match[1]))
				}
			}
		}
	}
}

func TestLocalMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(integrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run local gateway PostgreSQL tests", integrationDSNEnv)
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close PostgreSQL: %v", closeErr)
		}
	}()
	ctx := context.Background()
	schema := "idc_local_gateway_test_" + randomHex(t, 8)
	if _, err = database.ExecContext(ctx, `CREATE SCHEMA `+quoteIdentifier(schema)); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "idc_local_gateway_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
			return
		}
		if _, dropErr := database.ExecContext(ctx, `DROP SCHEMA `+quoteIdentifier(schema)+` CASCADE`); dropErr != nil {
			t.Errorf("drop isolated schema: %v", dropErr)
		}
	}()
	if _, err = database.ExecContext(ctx, `SET search_path TO `+quoteIdentifier(schema)); err != nil {
		t.Fatalf("set isolated search path: %v", err)
	}
	if err = Apply(ctx, database); err != nil {
		t.Fatalf("apply first migration: %v", err)
	}
	if err = Apply(ctx, database); err != nil {
		t.Fatalf("apply idempotent migration: %v", err)
	}
	migrations, loadErr := load()
	if loadErr != nil {
		t.Fatalf("load migrations for direct idempotency check: %v", loadErr)
	}
	for _, item := range migrations {
		if item.Name != "020-tencent-contract-status.sql" {
			continue
		}
		if _, err = database.ExecContext(ctx, item.SQL); err != nil {
			t.Fatalf("repeat migration %s directly: %v", item.Name, err)
		}
	}
	for _, table := range []string{
		"idc_gateway_local_business_request", "idc_gateway_process_nonce",
		"idc_gateway_supplier_operation",
		"idc_gateway_supplier_evidence", "idc_gateway_supplier_callback",
		"idc_gateway_supplier_circuit", "idc_gateway_supplier_takeover",
		"idc_gateway_supplier_plan_snapshot", "idc_gateway_supplier_usage_snapshot",
		"idc_gateway_supplier_budget_policy", "idc_gateway_supplier_budget_alert",
		"idc_gateway_supplier_reconciliation", "idc_gateway_supplier_sla_snapshot",
		"idc_gateway_supplier_collection_state",
		"idc_gateway_data_lifecycle_policy", "idc_gateway_legal_hold_event",
		"idc_gateway_evidence_export", "idc_gateway_supplier_authorization_event",
		"idc_gateway_restore_verification",
		"idc_gateway_supplier_processing_state",
		"idc_gateway_retention_disposition",
		"idc_gateway_local_supplier_instance",
		"idc_gateway_local_supplier_subject",
		"idc_gateway_local_subject_lookup",
		"idc_gateway_local_identity_operation",
		"idc_gateway_local_rate_limit_window",
		"idc_gateway_local_supplier_write_attempt",
		"idc_gateway_supplier_health_sample",
		"idc_gateway_local_supplier_route",
		"idc_gateway_local_callback_evidence",
		"idc_gateway_local_route_delivery",
		"idc_gateway_local_supplier_takeover",
		"idc_gateway_local_takeover_evidence",
		"idc_gateway_tencent_contract_report_job",
		"idc_gateway_schema_migration",
	} {
		assertTableExists(t, ctx, database, table, true)
	}
	for _, obsolete := range []string{
		"idc_gateway_registration_token", "idc_gateway_instance", "idc_gateway_instance_key",
		"idc_gateway_subject_binding", "idc_gateway_instance_delivery",
		"idc_gateway_price_version", "idc_gateway_price_item",
		"idc_gateway_settlement_account", "idc_gateway_settlement_hold",
		"idc_gateway_settlement_hold_line", "idc_gateway_settlement_usage_line",
		"idc_gateway_settlement_ledger_entry", "idc_gateway_provider_billing_gate",
		"idc_gateway_settlement_management_operation",
		"idc_gateway_settlement_management_account_result",
		"idc_gateway_settlement_management_price_result",
		"idc_gateway_settlement_management_gate_result",
	} {
		assertTableExists(t, ctx, database, obsolete, false)
	}
	var migrationCount int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM idc_gateway_schema_migration`).Scan(&migrationCount); err != nil || migrationCount != 21 {
		t.Fatalf("migration count=%d want=21 err=%v", migrationCount, err)
	}
	if err = Verify(ctx, database); err != nil {
		t.Fatalf("verify migration ledger: %v", err)
	}
	var evidenceCipherConstraint string
	if err = database.QueryRowContext(ctx, `
SELECT pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conname = 'ck_gateway_local_evidence_cipher'
  AND conrelid = 'idc_gateway_local_callback_evidence'::regclass`).Scan(&evidenceCipherConstraint); err != nil {
		t.Fatalf("read callback evidence cipher constraint: %v", err)
	}
	for _, required := range []string{"561168", "cipher_format_version = 1", "encryption_key_version > 0"} {
		if !strings.Contains(evidenceCipherConstraint, required) {
			t.Fatalf("callback evidence cipher constraint is missing %q: %s", required, evidenceCipherConstraint)
		}
	}
	assertDatabaseConstraintRejects(t, database, `
INSERT INTO idc_gateway_local_business_request
    (request_no, installation_id, operation_scope, method, request_target_sha256,
     idempotency_hmac, business_content_sha256, body_sha256)
VALUES ('lbr_00000000000000000000000000000000', 'invalid', 'installation.query', 'GET',
        decode(repeat('00', 32), 'hex'), decode(repeat('01', 32), 'hex'),
        decode(repeat('02', 32), 'hex'), decode(repeat('03', 32), 'hex'))`)
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_process_nonce
    (installation_id, nonce_sha256, frame_sha256, request_no, key_id, expires_at)
VALUES ('linst_1111111111111111111111111111111111111111111111111111111111111111',
        decode(repeat('21', 32), 'hex'), decode(repeat('22', 32), 'hex'), NULL,
        'lgk_1111111111111111', CURRENT_TIMESTAMP + INTERVAL '1 minute')`); err != nil {
		t.Fatalf("insert read-only nonce fence: %v", err)
	}
	assertDatabaseConstraintRejects(t, database, `
INSERT INTO idc_gateway_process_nonce
    (installation_id, nonce_sha256, frame_sha256, request_no, key_id, expires_at)
VALUES ('linst_1111111111111111111111111111111111111111111111111111111111111111',
        decode(repeat('21', 32), 'hex'), decode(repeat('23', 32), 'hex'), NULL,
        'lgk_1111111111111111', CURRENT_TIMESTAMP + INTERVAL '1 minute')`)
	assertDatabaseConstraintRejects(t, database, `
INSERT INTO idc_gateway_supplier_operation
    (operation_no, provider_code, operation_kind, idempotency_hmac, request_sha256)
VALUES ('lop_central_rejected', 'vendor_central_proxy', 'contract.create',
        decode(repeat('01', 32), 'hex'), decode(repeat('02', 32), 'hex'))`)
	assertDatabaseConstraintRejects(t, database, `
INSERT INTO idc_gateway_supplier_collection_state
    (provider_code, status_code, next_collect_at)
VALUES ('vendor_central_proxy', 'ready', CURRENT_TIMESTAMP)`)
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_operation
    (operation_no, provider_code, operation_kind, idempotency_hmac, request_sha256)
VALUES ('lop_test_1', 'tencent_essbasic', 'contract.create',
        decode(repeat('11', 32), 'hex'), decode(repeat('12', 32), 'hex'));
INSERT INTO idc_gateway_supplier_takeover
    (takeover_no, operation_no, employee_principal_no, reason_code,
     evidence_set_sha256, full_auth_at)
VALUES ('lto_test_1', 'lop_test_1', 'employee_1', 'result_unknown_review',
        decode(repeat('13', 32), 'hex'), CURRENT_TIMESTAMP);`); err != nil {
		t.Fatalf("insert local supplier fixtures: %v", err)
	}
	assertDatabaseConstraintRejects(t, database, `
UPDATE idc_gateway_supplier_takeover SET reason_code = 'changed'
WHERE takeover_no = 'lto_test_1'`)
}

func assertTableExists(t *testing.T, ctx context.Context, database *sql.DB, table string, want bool) {
	t.Helper()
	var exists bool
	if err := database.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil || exists != want {
		t.Fatalf("table %s exists=%t want=%t err=%v", table, exists, want, err)
	}
}

func assertDatabaseConstraintRejects(t *testing.T, database *sql.DB, statement string) {
	t.Helper()
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin rejecting transaction: %v", err)
	}
	if _, err = transaction.Exec(statement); err == nil {
		if commitErr := transaction.Commit(); commitErr == nil {
			t.Fatalf("expected statement rejection: %s", statement)
		}
		return
	}
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		t.Fatalf("rollback rejected statement: %v", rollbackErr)
	}
}

func randomHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatalf("generate random suffix: %v", err)
	}
	return hex.EncodeToString(value)
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
