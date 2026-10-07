package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestSimpleInventoryDecisionMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL simple inventory decision tests", catalogIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open inventory migration database: %v", err)
	}
	schema := newCommerceIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatalf("create inventory migration schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatalf("open scoped inventory migration database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, simpleInventoryDecisionFixtureSQL); err != nil {
		t.Fatalf("create inventory decision fixtures: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve inventory migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "093-simple-inventory-fail-closed.sql"))
	if err != nil {
		t.Fatalf("read inventory migration: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_item_snapshot(inventory_decision_code,route_decision_digest) VALUES('manual_confirmation',repeat('1',64))`); err != nil {
		t.Fatalf("seed legacy inventory decision: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err == nil {
		t.Fatal("migration accepted legacy manual inventory evidence")
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_commerce_quote_item_snapshot`); err != nil {
		t.Fatalf("remove legacy development fixture: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply inventory migration attempt %d: %v", attempt, err)
		}
	}
	if value, queryErr := database.GetValue(ctx, `SELECT count(*) FROM sys_dict_data WHERE dict_type='idc_commerce_inventory_decision' AND value='manual_confirmation'`); queryErr != nil || value.Int() != 0 {
		t.Fatalf("legacy inventory dictionary count=%d err=%v", value.Int(), queryErr)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_item_snapshot(inventory_decision_code,route_decision_digest) VALUES('manual_confirmation',repeat('2',64))`); err == nil {
		t.Fatal("quote constraint accepted manual inventory decision")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_item_snapshot(inventory_decision_code,route_decision_digest) VALUES('not_required',repeat('5',64))`); err != nil {
		t.Fatalf("renewal quote inventory exemption was rejected: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_order_item(fulfillment_mode_code,risk_decision_code,inventory_decision_code,route_decision_digest) VALUES('automatic','approved','manual_confirmation',repeat('3',64))`); err == nil {
		t.Fatal("order constraint accepted manual inventory decision")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_order_item(fulfillment_mode_code,risk_decision_code,inventory_decision_code,route_decision_digest) VALUES('automatic','approved','not_required',repeat('6',64))`); err == nil {
		t.Fatal("new-purchase order constraint accepted renewal inventory exemption")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_order_item(fulfillment_mode_code,risk_decision_code,inventory_decision_code,route_decision_digest) VALUES('manual_review','approved','available',repeat('4',64))`); err != nil {
		t.Fatalf("explicit fulfillment review was rejected: %v", err)
	}
}

const simpleInventoryDecisionFixtureSQL = `
CREATE TABLE plugin_idc_core_commerce_quote_item_snapshot (
    inventory_decision_code VARCHAR(24) NOT NULL,
    route_decision_digest CHAR(64) NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item (
    fulfillment_mode_code VARCHAR(16) NOT NULL,
    risk_decision_code VARCHAR(16) NOT NULL,
    inventory_decision_code VARCHAR(24) NOT NULL,
    route_decision_digest CHAR(64) NOT NULL
);
CREATE TABLE sys_dict_data (
    dict_type VARCHAR(100) NOT NULL,
    value VARCHAR(100) NOT NULL
);
INSERT INTO sys_dict_data(dict_type,value)
VALUES('idc_commerce_inventory_decision','manual_confirmation');`
