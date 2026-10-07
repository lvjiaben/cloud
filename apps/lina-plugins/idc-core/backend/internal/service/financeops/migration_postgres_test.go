package financeops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const financialOperationsTestDSN = "LINAPRO_IDC_CORE_TEST_DSN"

func TestFinancialOperationsMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(financialOperationsTestDSN))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL financial operations migration tests", financialOperationsTestDSN)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 8)
	if _, err = rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_finops_test_" + hex.EncodeToString(raw)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_finops_test_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, financialOperationsHostFixture); err != nil {
		t.Fatalf("create host fixtures: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	paths, err := filepath.Glob(filepath.Join(root, "[0-9][0-9][0-9]-*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("execute %s: %v", filepath.Base(path), err)
		}
	}
	last, err := os.ReadFile(filepath.Join(root, "050-financial-operations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(last)); err != nil {
		t.Fatalf("replay migration: %v", err)
	}
	for _, table := range []string{"plugin_idc_core_financial_delivery_intent", "plugin_idc_core_financial_internal_note", "plugin_idc_core_financial_saved_filter", "plugin_idc_core_financial_export_request", "plugin_idc_core_financial_export_artifact", "plugin_idc_core_financial_export_ticket"} {
		value, queryErr := database.GetValue(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?`, schema, table)
		if queryErr != nil || value.Int() != 1 {
			t.Fatalf("missing table %s: %v", table, queryErr)
		}
	}
}

func appendSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + url.QueryEscape(schema)
}

const financialOperationsHostFixture = `
CREATE TABLE sys_dict_type (id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,name VARCHAR(128) NOT NULL DEFAULT '',type VARCHAR(128) NOT NULL DEFAULT '',status SMALLINT NOT NULL DEFAULT 1,is_builtin SMALLINT NOT NULL DEFAULT 0,allow_tenant_override BOOL NOT NULL DEFAULT FALSE,remark VARCHAR(512) NOT NULL DEFAULT '',created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,CONSTRAINT uk_finops_dict_type UNIQUE (tenant_id,type));
CREATE TABLE sys_dict_data (id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,dict_type VARCHAR(128) NOT NULL DEFAULT '',label VARCHAR(128) NOT NULL DEFAULT '',value VARCHAR(128) NOT NULL DEFAULT '',sort INT NOT NULL DEFAULT 0,tag_style VARCHAR(128) NOT NULL DEFAULT '',status SMALLINT NOT NULL DEFAULT 1,is_builtin SMALLINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,CONSTRAINT uk_finops_dict_data UNIQUE (tenant_id,dict_type,value));`
