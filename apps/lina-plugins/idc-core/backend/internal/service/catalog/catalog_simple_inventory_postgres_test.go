// This file verifies the product-neutral inventory schema, migration
// idempotence, constraints, and concurrent non-overselling behavior in real
// PostgreSQL.
package catalog

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"lina-core/pkg/bizerr"
)

const simpleInventoryVersionNo = "ver_99999999999999999999999999999999"

func TestSimpleInventoryPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}

	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version (
	tenant_id INT NOT NULL DEFAULT 0,
	version_no VARCHAR(40) PRIMARY KEY,
	status_code VARCHAR(32) NOT NULL DEFAULT 'draft'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(version_no) VALUES (?)`, simpleInventoryVersionNo); err != nil {
		t.Fatal(err)
	}

	migration, err := os.ReadFile(simpleInventoryMigrationPath(t))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply inventory migration attempt %d: %v", attempt+1, err)
		}
	}
	columnCount, err := database.GetValue(ctx, `SELECT count(*) FROM information_schema.columns
WHERE table_schema=current_schema() AND table_name='plugin_idc_core_catalog_product_version_inventory'
  AND column_name IN('version_no','tracking_enabled','sellable_quantity','version','updated_at')`)
	if err != nil || columnCount.Int() != 5 {
		t.Fatalf("inventory minimal columns: count=%d err=%v", columnCount.Int(), err)
	}
	allColumnCount, err := database.GetValue(ctx, `SELECT count(*) FROM information_schema.columns
WHERE table_schema=current_schema() AND table_name='plugin_idc_core_catalog_product_version_inventory'`)
	if err != nil || allColumnCount.Int() != 5 {
		t.Fatalf("inventory contains unexpected columns: count=%d err=%v", allColumnCount.Int(), err)
	}
	indexCount, err := database.GetValue(ctx, `SELECT count(*) FROM pg_indexes
WHERE schemaname=current_schema() AND tablename='plugin_idc_core_catalog_product_version_inventory'`)
	if err != nil || indexCount.Int() != 1 {
		t.Fatalf("inventory exact primary-key index: count=%d err=%v", indexCount.Int(), err)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
 SET tracking_enabled=TRUE,sellable_quantity=7 WHERE version_no=?`, simpleInventoryVersionNo); err != nil {
		t.Fatal(err)
	}

	var successes atomic.Int64
	var wait sync.WaitGroup
	for contender := 0; contender < 24; contender++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET sellable_quantity=sellable_quantity-1,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE version_no=? AND tracking_enabled=TRUE AND sellable_quantity>=1`, simpleInventoryVersionNo)
			if updateErr != nil {
				t.Errorf("atomic inventory decrement: %v", updateErr)
				return
			}
			rows, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				t.Errorf("inventory decrement rows affected: %v", rowsErr)
				return
			}
			if rows == 1 {
				successes.Add(1)
			}
		}()
	}
	wait.Wait()

	var stored struct {
		SellableQuantity int64 `json:"sellable_quantity"`
		Version          int64 `json:"version"`
	}
	if err = database.GetScan(ctx, &stored, `SELECT sellable_quantity,version
FROM plugin_idc_core_catalog_product_version_inventory WHERE version_no=?`, simpleInventoryVersionNo); err != nil {
		t.Fatal(err)
	}
	if successes.Load() != 7 || stored.SellableQuantity != 0 || stored.Version != 8 {
		t.Fatalf("inventory oversold: successes=%d stored=%+v", successes.Load(), stored)
	}

	result, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET sellable_quantity=3,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE version_no=? AND version=?`, simpleInventoryVersionNo, stored.Version)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		t.Fatalf("current inventory version update: rows=%d err=%v", rows, err)
	}
	stale, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET sellable_quantity=2,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE version_no=? AND version=?`, simpleInventoryVersionNo, stored.Version)
	if err != nil {
		t.Fatal(err)
	}
	staleRows, err := stale.RowsAffected()
	if err != nil || staleRows != 0 {
		t.Fatalf("stale inventory version update: rows=%d err=%v", staleRows, err)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET sellable_quantity=-1 WHERE version_no=?`, simpleInventoryVersionNo); err == nil {
		t.Fatal("negative sellable quantity was accepted")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET version=0 WHERE version_no=?`, simpleInventoryVersionNo); err == nil {
		t.Fatal("non-positive inventory version was accepted")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_inventory
 (version_no) VALUES ('ver_88888888888888888888888888888888')`); err == nil {
		t.Fatal("inventory without a product version was accepted")
	}

	service, serviceErr := newManagementService(&managementTestAudit{}, &managementTestAuthorizer{actor: 7}, strings.NewReader(strings.Repeat("i", 16)))
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	updated, serviceErr := service.UpdateProductVersionInventory(ctx, ProductVersionInventoryUpdateInput{
		VersionNo: simpleInventoryVersionNo, TrackingEnabled: true,
		SellableQuantity: 12, ExpectedVersion: stored.Version + 1,
	})
	if serviceErr != nil || updated.SellableQuantity != 12 || updated.Version != stored.Version+2 {
		t.Fatalf("production inventory update=%+v err=%v", updated, serviceErr)
	}
	if _, staleErr := service.UpdateProductVersionInventory(ctx, ProductVersionInventoryUpdateInput{
		VersionNo: simpleInventoryVersionNo, TrackingEnabled: true,
		SellableQuantity: 13, ExpectedVersion: stored.Version + 1,
	}); !bizerr.Is(staleErr, CodeManagementConflict) {
		t.Fatalf("stale production inventory update error=%v", staleErr)
	}
}

func simpleInventoryMigrationPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve inventory test file")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "081-simple-product-version-inventory.sql")
}
