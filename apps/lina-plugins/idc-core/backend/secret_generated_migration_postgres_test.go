// This file verifies the generated-secret request fence migration in the
// disposable DAO code-generation database.

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

func TestGeneratedSecretCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare generated-secret DAO metadata", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create generated-secret code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close generated-secret code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve generated-secret migration test path")
	}
	path := filepath.Join(
		filepath.Dir(filename), "..", "manifest", "sql", "039-generated-secret-policy.sql",
	)
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated-secret code-generation migration: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute generated-secret code-generation migration attempt %d: %v", attempt, err)
		}
	}
	description, err := database.GetValue(ctx, `
SELECT col_description(
    'plugin_idc_core_secret_version'::regclass,
    (SELECT ordinal_position
     FROM information_schema.columns
     WHERE table_schema = current_schema()
       AND table_name = 'plugin_idc_core_secret_version'
       AND column_name = 'request_operation')
)`)
	if err != nil || description.String() !=
		"Authenticated write operation discriminator: create, generate_u256, or update" {
		t.Fatalf("generated-secret operation description=%q err=%v", description.String(), err)
	}
}
