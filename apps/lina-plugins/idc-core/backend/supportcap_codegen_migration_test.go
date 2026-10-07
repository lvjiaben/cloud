// This file keeps anonymous support, custom-field, and related-entitlement SQL metadata aligned in the isolated code-generation database.
package backend

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestSupportcapCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare supportcap DAO metadata", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("connect supportcap code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close supportcap code-generation database: %v", closeErr)
		}
	})
	databaseName, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || databaseName.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", databaseName.String(), err)
	}
	schema, err := database.GetValue(ctx, "SELECT current_schema()")
	if err != nil || schema.String() != "public" {
		t.Fatalf("refusing non-public codegen schema %q: %v", schema.String(), err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve supportcap migration path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	migration, err := os.ReadFile(filepath.Join(migrationRoot, "042-support-content.sql"))
	if err != nil {
		t.Fatalf("read supportcap migration: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute supportcap migration attempt %d: %v", attempt, err)
		}
		entitlementMigration, readErr := os.ReadFile(filepath.Join(migrationRoot, "091-support-entitlement-association.sql"))
		if readErr != nil {
			t.Fatalf("read support entitlement migration: %v", readErr)
		}
		if _, err = database.Exec(ctx, string(entitlementMigration)); err != nil {
			t.Fatalf("execute support entitlement migration attempt %d: %v", attempt, err)
		}
	}
	columns, err := database.GetValue(ctx, `SELECT COUNT(1) FROM information_schema.columns WHERE table_schema='public' AND table_name='plugin_idc_core_support_ticket_access_link' AND column_name IN ('operation_digest','request_digest','token_secret_ref')`)
	if err != nil || columns.Int() != 3 {
		t.Fatalf("supportcap operation columns=%d want=3 err=%v", columns.Int(), err)
	}
	checks, err := database.GetValue(ctx, `SELECT COUNT(1) FROM pg_constraint WHERE conrelid='plugin_idc_core_support_ticket_access_link'::regclass AND conname='ck_idc_support_ticket_access_link_operation'`)
	if err != nil || checks.Int() != 1 {
		t.Fatalf("supportcap operation constraint=%d want=1 err=%v", checks.Int(), err)
	}
	indexes, err := database.GetValue(ctx, `SELECT COUNT(1) FROM pg_indexes WHERE schemaname='public' AND tablename='plugin_idc_core_support_ticket_access_link' AND indexname='uk_idc_support_ticket_access_link_operation'`)
	if err != nil || indexes.Int() != 1 {
		t.Fatalf("supportcap operation index=%d want=1 err=%v", indexes.Int(), err)
	}
	customFieldTables, err := database.GetValue(ctx, `SELECT COUNT(1) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('plugin_idc_core_support_custom_field_definition','plugin_idc_core_support_custom_field_option','plugin_idc_core_support_ticket_custom_field_value','plugin_idc_core_support_ticket_custom_field_value_option')`)
	if err != nil || customFieldTables.Int() != 4 {
		t.Fatalf("support custom field tables=%d want=4 err=%v", customFieldTables.Int(), err)
	}
	entitlementSnapshotColumns, err := database.GetValue(ctx, `SELECT COUNT(1) FROM information_schema.columns WHERE table_schema='public' AND table_name='plugin_idc_core_support_ticket' AND column_name IN ('related_entitlement_no','related_entitlement_version','related_entitlement_name_zh_cn','related_entitlement_name_en_us','related_entitlement_status')`)
	if err != nil || entitlementSnapshotColumns.Int() != 5 {
		t.Fatalf("support entitlement snapshot columns=%d want=5 err=%v", entitlementSnapshotColumns.Int(), err)
	}
	associationColumns, err := database.GetValue(ctx, `SELECT COUNT(1) FROM information_schema.columns WHERE table_schema='public' AND table_name='plugin_idc_core_service_entitlement' AND column_name='support_association_reference' AND is_generated='ALWAYS'`)
	if err != nil || associationColumns.Int() != 1 {
		t.Fatalf("support association generated columns=%d want=1 err=%v", associationColumns.Int(), err)
	}
	associationIndexes, err := database.GetValue(ctx, `SELECT COUNT(1) FROM pg_indexes WHERE schemaname='public' AND tablename='plugin_idc_core_service_entitlement' AND indexname='uk_idc_core_service_entitlement_support_association_reference'`)
	if err != nil || associationIndexes.Int() != 1 {
		t.Fatalf("support association indexes=%d want=1 err=%v", associationIndexes.Int(), err)
	}
	legacyColumns, err := database.GetValue(ctx, `SELECT COUNT(1) FROM information_schema.columns WHERE table_schema='public' AND table_name='plugin_idc_core_support_ticket' AND column_name LIKE 'related_service_%'`)
	if err != nil || legacyColumns.Int() != 0 {
		t.Fatalf("legacy support service columns=%d want=0 err=%v", legacyColumns.Int(), err)
	}
}
