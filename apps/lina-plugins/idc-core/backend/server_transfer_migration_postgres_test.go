package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestServerTransferMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run server transfer PostgreSQL tests", catalogIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_billing_test_") {
			_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		} else {
			t.Errorf("refusing to drop unexpected transfer schema %q", schema)
		}
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(
		dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, transferHostFixture); err != nil {
		t.Fatalf("create host fixtures: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transfer migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
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
	for _, table := range []string{
		"plugin_idc_core_service_transfer_request",
		"plugin_idc_core_service_ownership_event",
		"plugin_idc_core_server_resize_quote",
		"plugin_idc_core_server_resize_intent",
		"plugin_idc_core_server_resize_refund_handoff",
	} {
		value, queryErr := database.GetValue(ctx, `SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema=? AND table_name=?`, schema, table)
		if queryErr != nil || value.Int() != 1 {
			t.Fatalf("missing table %s: %v", table, queryErr)
		}
	}
}

const transferHostFixture = `
CREATE TABLE sys_dict_type (id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,name VARCHAR(128) NOT NULL DEFAULT '',type VARCHAR(128) NOT NULL DEFAULT '',status SMALLINT NOT NULL DEFAULT 1,is_builtin SMALLINT NOT NULL DEFAULT 0,allow_tenant_override BOOL NOT NULL DEFAULT FALSE,remark VARCHAR(512) NOT NULL DEFAULT '',created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,CONSTRAINT uk_transfer_dict_type UNIQUE (tenant_id,type));
CREATE TABLE sys_dict_data (id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,dict_type VARCHAR(128) NOT NULL DEFAULT '',label VARCHAR(128) NOT NULL DEFAULT '',value VARCHAR(128) NOT NULL DEFAULT '',sort INT NOT NULL DEFAULT 0,tag_style VARCHAR(128) NOT NULL DEFAULT '',status SMALLINT NOT NULL DEFAULT 1,is_builtin SMALLINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,CONSTRAINT uk_transfer_dict_data UNIQUE (tenant_id,dict_type,value));`
