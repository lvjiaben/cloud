// This file verifies the latest settlement, fulfillment, and provisioning schema.

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

// TestSettlementFulfillmentCodegenPostgreSQL applies the latest migrations only
// to the explicitly named disposable DAO code-generation database.
func TestSettlementFulfillmentCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare the settlement DAO code-generation database", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create settlement code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close settlement code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve settlement migration test path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	for _, name := range []string{
		"036-trial-qualification.sql",
		"037-financial-settlement-fulfillment.sql",
		"038-service-provision-execution.sql",
	} {
		migration, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatalf("read code-generation migration %s: %v", name, readErr)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute code-generation migration %s attempt %d: %v", name, attempt+1, err)
			}
		}
	}
}
