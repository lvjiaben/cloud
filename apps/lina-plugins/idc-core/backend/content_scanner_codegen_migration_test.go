// content_scanner_codegen_migration_test.go checks scanner SQL and generated models stay aligned.
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

func TestContentScannerCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare content scanner DAO metadata", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("connect content scanner code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close content scanner code-generation database: %v", closeErr)
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
		t.Fatal("resolve content scanner migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "042-support-content.sql"))
	if err != nil {
		t.Fatalf("read content scanner migration: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute content scanner migration attempt %d: %v", attempt, err)
		}
	}
	columns, err := database.GetValue(ctx, `SELECT COUNT(1) FROM information_schema.columns WHERE table_schema='public' AND ((table_name='plugin_idc_core_content_file' AND column_name='object_deleted_at') OR (table_name='plugin_idc_core_content_scan_attempt' AND column_name IN ('queued_at','lease_expires_at','version')) OR (table_name='plugin_idc_core_content_upload_session' AND column_name IN ('cleanup_lease_expires_at','cleanup_completed_at')))`)
	if err != nil || columns.Int() != 6 {
		t.Fatalf("content scanner codegen columns=%d want=6 err=%v", columns.Int(), err)
	}
}
