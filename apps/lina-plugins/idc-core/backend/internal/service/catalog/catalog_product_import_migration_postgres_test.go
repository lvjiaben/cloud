// This file verifies immutable catalog-import evidence, tenant/product/version
// ownership, bounded source projections, and transaction-only reservations in
// a disposable PostgreSQL schema. It never changes installed product data.

package catalog

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestCatalogProductImportMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, catalogImportMigrationFixtureSQL); err != nil {
		t.Fatal(err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve import migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "110-catalog-product-import.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply import migration attempt %d: %v", attempt+1, err)
		}
	}

	// A reservation is not a durable result until the same transaction binds
	// both local identities. Its source projection stays unchanged throughout.
	err = database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		if _, insertErr := database.Exec(txctx, catalogImportMigrationInsertSQL,
			"request-1", catalogImportMigrationSourceJSON); insertErr != nil {
			return insertErr
		}
		_, updateErr := database.Exec(txctx, `UPDATE plugin_idc_core_catalog_product_import
SET product_no='prd_11111111111111111111111111111111',version_no='ver_11111111111111111111111111111111'
WHERE request_key='request-1'`)
		return updateErr
	})
	if err != nil {
		t.Fatalf("atomically bind original draft: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("replay migration with durable evidence: %v", err)
	}
	count, err := database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product_import
WHERE request_key='request-1' AND product_no='prd_11111111111111111111111111111111'`)
	if err != nil || count != 1 {
		t.Fatalf("migration lost import evidence: count=%d err=%v", count, err)
	}
	assertCatalogImportConcurrentReplay(t, ctx, database)
	for _, scenario := range []struct {
		name string
		sql  string
	}{
		{"changed request", `UPDATE plugin_idc_core_catalog_product_import SET request_digest=repeat('b',64)`},
		{"changed source", `UPDATE plugin_idc_core_catalog_product_import SET source_snapshot=jsonb_set(source_snapshot,'{product,name}','"Replacement"')`},
		{"changed source gaps", `UPDATE plugin_idc_core_catalog_product_import SET missing_configuration_snapshot='[{"fieldKey":"cpu","nameZhCn":"CPU","nameEnUs":"CPU"}]'`},
		{"changed actor", `UPDATE plugin_idc_core_catalog_product_import SET actor_user_id=8`},
		{"changed draft", `UPDATE plugin_idc_core_catalog_product_import SET product_no='prd_22222222222222222222222222222222',version_no='ver_22222222222222222222222222222222'`},
		{"evidence deletion", `DELETE FROM plugin_idc_core_catalog_product_import`},
	} {
		if _, mutationErr := database.Exec(ctx, scenario.sql); mutationErr == nil {
			t.Fatalf("accepted %s", scenario.name)
		}
	}
	if _, err = database.Exec(ctx, catalogImportMigrationInsertSQL,
		"dangling-request", catalogImportMigrationSourceJSON); err == nil ||
		!strings.Contains(err.Error(), "binding is incomplete") {
		t.Fatalf("committed incomplete reservation: %v", err)
	}
	assertCatalogImportProjectionValidation(t, ctx, database)
	assertCatalogImportOwnershipValidation(t, ctx, database)
	assertCatalogImportTriggerIdentity(t, ctx, database, string(migration))

	// Publication belongs to the existing product workflow. Import evidence
	// remains unchanged and cannot prevent that later, explicitly requested step.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published'
WHERE version_no='ver_11111111111111111111111111111111'`); err != nil {
		t.Fatalf("later draft publication was blocked by evidence: %v", err)
	}
}

func assertCatalogImportTriggerIdentity(t *testing.T, ctx context.Context, database gdb.DB, migration string) {
	t.Helper()
	// A same-name disabled trigger must not make a replay falsely succeed.
	// Returning the migration error rolls the fixture-only mutation back.
	err := database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		if _, disableErr := database.Exec(txctx, `ALTER TABLE plugin_idc_core_catalog_product_import
DISABLE TRIGGER trg_plugin_idc_core_catalog_import_complete`); disableErr != nil {
			return disableErr
		}
		_, replayErr := database.Exec(txctx, migration)
		return replayErr
	})
	if err == nil || !strings.Contains(err.Error(), "catalog contract is invalid") {
		t.Fatalf("migration accepted an invalid deferred trigger: %v", err)
	}
	if _, err = database.Exec(ctx, migration); err != nil {
		t.Fatalf("migration replay after rolled-back trigger drift: %v", err)
	}
}

func assertCatalogImportConcurrentReplay(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := database.Exec(ctx, catalogImportMigrationInsertSQL+`
ON CONFLICT (tenant_id,actor_user_id,request_key) DO NOTHING`, "request-1", catalogImportMigrationSourceJSON)
			if err != nil {
				t.Errorf("concurrent replay failed: %v", err)
				return
			}
			affected, err := result.RowsAffected()
			if err != nil || affected != 0 {
				t.Errorf("replay overwrote or duplicated the import: affected=%d err=%v", affected, err)
			}
		}()
	}
	wait.Wait()
}

func assertCatalogImportProjectionValidation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for _, scenario := range []struct {
		name       string
		expression string
	}{
		{"raw response", `$1::jsonb || '{"rawResponse":{"secret":"never"}}'::jsonb`},
		{"nested secret", `jsonb_set($1::jsonb,'{product,token}','"never"')`},
		{"null array", `jsonb_set($1::jsonb,'{options}','null')`},
		{"incomplete price marked complete", `jsonb_set($1::jsonb,'{priceScheduleComplete}','true')`},
		{"arbitrary field JSON", `jsonb_set($1::jsonb,'{productConfiguration}','[{"fieldKey":"cpu","textValue":{"secret":"never"}}]')`},
		{"multi-value field", `jsonb_set($1::jsonb,'{productConfiguration}','[{"fieldKey":"cpu","integerValue":4,"textValue":"4"}]')`},
		{"missing period", `jsonb_set($1::jsonb,'{basePrices}','[{"currencyCode":"CNY","period":null,"periodCount":1,"amount":"0","setupFee":"0"}]')`},
	} {
		valid, err := database.GetValue(ctx, `SELECT plugin_idc_core_import_source_valid(`+scenario.expression+`)`, catalogImportMigrationSourceJSON)
		if err != nil || valid.Bool() {
			t.Fatalf("projection %s: valid=%v err=%v", scenario.name, valid, err)
		}
	}
	valid, err := database.GetValue(ctx, `SELECT plugin_idc_core_import_source_valid(jsonb_set($1::jsonb,'{productConfiguration}',
'[{"fieldKey":"cpu","integerValue":4},{"fieldKey":"enabled","booleanValue":false},{"fieldKey":"name","textValue":""}]'))`, catalogImportMigrationSourceJSON)
	if err != nil || !valid.Bool() {
		t.Fatalf("typed zero/false/empty values rejected: valid=%v err=%v", valid, err)
	}
}

func assertCatalogImportOwnershipValidation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for _, scenario := range []struct {
		name    string
		columns string
		values  string
	}{
		{"duplicate request", "product_no,version_no", "'prd_22222222222222222222222222222222','ver_22222222222222222222222222222222'"},
		{"duplicate version", "product_no,version_no", "'prd_11111111111111111111111111111111','ver_11111111111111111111111111111111'"},
		{"cross product version", "product_no,version_no", "'prd_11111111111111111111111111111111','ver_22222222222222222222222222222222'"},
		{"cross tenant version", "product_no,version_no", "'prd_33333333333333333333333333333333','ver_33333333333333333333333333333333'"},
		{"already published", "product_no,version_no", "'prd_44444444444444444444444444444444','ver_44444444444444444444444444444444'"},
	} {
		requestKey := scenario.name
		if scenario.name == "duplicate request" {
			requestKey = "request-1"
		}
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_import
(tenant_id,actor_user_id,request_key,request_digest,interface_instance_no,upstream_reference,source_snapshot,pricing_incomplete,`+scenario.columns+`)
VALUES(0,7,?,repeat('a',64),'pfi_11111111111111111111111111111111','upstream-1',?::jsonb,true,`+scenario.values+`)`, requestKey, catalogImportMigrationSourceJSON)
		if err == nil {
			t.Fatalf("accepted %s", scenario.name)
		}
	}
}

const catalogImportMigrationSourceJSON = `{"product":{"reference":"upstream-1","name":"Upstream product","description":""},"basePrices":[],"options":[],"priceScheduleComplete":false,"productConfiguration":[]}`

const catalogImportMigrationInsertSQL = `INSERT INTO plugin_idc_core_catalog_product_import
(tenant_id,actor_user_id,request_key,request_digest,interface_instance_no,upstream_reference,source_snapshot,pricing_incomplete)
VALUES(0,7,?,repeat('a',64),'pfi_11111111111111111111111111111111','upstream-1',?::jsonb,true)`

const catalogImportMigrationFixtureSQL = `
CREATE TABLE plugin_idc_core_catalog_product(tenant_id INT NOT NULL,product_no VARCHAR(40) PRIMARY KEY,status_code TEXT NOT NULL);
CREATE TABLE plugin_idc_core_catalog_product_version(tenant_id INT NOT NULL,product_no VARCHAR(40) NOT NULL,version_no VARCHAR(40) PRIMARY KEY,status_code TEXT NOT NULL);
CREATE TABLE plugin_idc_core_product_automation_interface_instance(tenant_id INT NOT NULL,interface_instance_no VARCHAR(40) NOT NULL,UNIQUE(tenant_id,interface_instance_no));
INSERT INTO plugin_idc_core_product_automation_interface_instance VALUES(0,'pfi_11111111111111111111111111111111');
INSERT INTO plugin_idc_core_catalog_product VALUES
(0,'prd_11111111111111111111111111111111','draft'),(0,'prd_22222222222222222222222222222222','draft'),
(9,'prd_33333333333333333333333333333333','draft'),(0,'prd_44444444444444444444444444444444','active');
INSERT INTO plugin_idc_core_catalog_product_version VALUES
(0,'prd_11111111111111111111111111111111','ver_11111111111111111111111111111111','draft'),
(0,'prd_22222222222222222222222222222222','ver_22222222222222222222222222222222','draft'),
(9,'prd_33333333333333333333333333333333','ver_33333333333333333333333333333333','draft'),
(0,'prd_44444444444444444444444444444444','ver_44444444444444444444444444444444','published');`
