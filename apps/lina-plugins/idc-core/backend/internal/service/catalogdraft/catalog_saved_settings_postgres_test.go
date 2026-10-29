// This file verifies immediate settings application, rollback and off-sale
// preservation against real PostgreSQL, without supplier or publication gates.
package catalogdraft

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestSavedProductSettingsApplyAtomicallyPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	installAutomationPublicationMigration(t, ctx)
	insertUnifiedPublicationCatalog(t, ctx)
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/081-simple-product-version-inventory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	authority := &catalogProductPublicationTestAuthorizer{actorUserID: 17}
	service, err := catalog.NewPublication(&catalogDraftTestAuditWriter{}, authority, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.PublishVersion(ctx, "version-manual-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory SET tracking_enabled=TRUE,sellable_quantity=9 WHERE version_no='version-manual-1'`); err != nil {
		t.Fatal(err)
	}
	oldPrice, err := g.DB().GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no='version-manual-1' ORDER BY id LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
	failing, err := catalog.NewPublication(&catalogDraftTestAuditWriter{err: errors.New("injected audit failure")}, authority, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	save := func(ctx context.Context) error {
		_, writeErr := g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_price SET amount_minor=12345 WHERE version_no='version-copy-2'`)
		return writeErr
	}
	before, err := g.DB().GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no='version-copy-2' ORDER BY id LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
	if err = failing.WithSavedSettings(ctx, "version-copy-2", save); err == nil {
		t.Fatal("failed save reported success")
	}
	assertUnifiedPublicationState(t, ctx, "published", "draft", "version-manual-1")
	after, err := g.DB().GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no='version-copy-2' ORDER BY id LIMIT 1`)
	if err != nil || after.Int64() != before.Int64() {
		t.Fatalf("partial price survived failure: %v", err)
	}
	if err = service.WithSavedSettings(ctx, "version-copy-2", save); err != nil {
		t.Fatal(err)
	}
	assertUnifiedPublicationState(t, ctx, "superseded", "published", "version-copy-2")
	row, err := g.DB().GetOne(ctx, `SELECT i.sellable_quantity,p.amount_minor FROM plugin_idc_core_catalog_product_version_inventory i JOIN plugin_idc_core_catalog_price p USING(version_no) WHERE i.version_no='version-copy-2' ORDER BY p.id LIMIT 1`)
	if err != nil || row["sellable_quantity"].Int64() != 9 || row["amount_minor"].Int64() != 12345 {
		t.Fatalf("saved price/stock not applied: %v %v", row, err)
	}
	historical, err := g.DB().GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE version_no='version-manual-1' ORDER BY id LIMIT 1`)
	if err != nil || historical.Int64() != oldPrice.Int64() {
		t.Fatal("historical price changed")
	}
	// Incomplete new settings can be saved. They remain off sale; actual sales
	// still use the existing completeness and provider checks on the sale switch.
	if _, err = service.ApplySavedVersion(ctx, "version-incomplete-1"); err != nil {
		t.Fatalf("saving incomplete settings was blocked: %v", err)
	}
	state, err := g.DB().GetValue(ctx, `SELECT status_code FROM plugin_idc_core_catalog_product WHERE product_no='product-incomplete'`)
	if err != nil || state.String() != "suspended" {
		t.Fatal("save enabled an incomplete product")
	}
	if _, err = g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='product-unified'; INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code) VALUES('version-settings-3','product-unified',3,'draft','manual'); INSERT INTO plugin_idc_core_catalog_product_version_inventory(version_no,tracking_enabled,sellable_quantity,version) VALUES('version-settings-3',TRUE,4,2)`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ApplySavedVersion(ctx, "version-settings-3"); err != nil {
		t.Fatal(err)
	}
	row, err = g.DB().GetOne(ctx, `SELECT p.status_code,i.sellable_quantity FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_product_version_inventory i ON i.version_no=p.current_version_no WHERE p.product_no='product-unified'`)
	if err != nil || row["status_code"].String() != "suspended" || row["sellable_quantity"].Int64() != 4 {
		t.Fatalf("saving overwrote sale switch or explicit stock: %v %v", row, err)
	}
}
