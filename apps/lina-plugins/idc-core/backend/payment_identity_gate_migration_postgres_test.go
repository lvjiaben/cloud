// This file verifies the payment identity-gate migration in PostgreSQL.

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

// TestPaymentIdentityGateCodegenPostgreSQL applies only the additive payment
// gate migration to the explicitly named disposable DAO code-generation database.
func TestPaymentIdentityGateCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare the payment identity-gate DAO code-generation database", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create payment identity-gate code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close payment identity-gate code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve payment identity-gate migration path")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "025-commerce-payment-identity-gate.sql")
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payment identity-gate migration: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute payment identity-gate migration attempt %d: %v", attempt+1, err)
		}
	}
}
