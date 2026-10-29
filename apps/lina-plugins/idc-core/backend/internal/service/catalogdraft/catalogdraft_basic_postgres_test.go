// This file verifies the combined basic form uses one real PostgreSQL transaction.
// A late section failure cannot leave metadata, companion offers or stock changed.
package catalogdraft

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type basicTestAuthorizer struct {
	*copyTestAuthorizer
	denyStock bool
}

func (a *basicTestAuthorizer) AuthorizeProductCreate(ctx context.Context) (int, error) {
	if a.denyStock {
		return 0, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.copyTestAuthorizer.AuthorizeProductCreate(ctx)
}

type basicTestAutomation struct {
	productautomation.Service
	received productautomation.SaveVersionConfigurationInput
}

func (a *basicTestAutomation) GetMerchantConfiguration(context.Context, string) (*productautomation.VersionConfiguration, error) {
	region := "r1"
	return &productautomation.VersionConfiguration{Version: 3, MerchantValues: []productautomation.ConfigurationValue{{FieldKey: "region", TextValue: &region}}, CustomerFieldSelections: []productautomation.CustomerFieldSelection{{FieldKey: "cpu"}}}, nil
}
func (a *basicTestAutomation) SaveVersionConfiguration(_ context.Context, input productautomation.SaveVersionConfigurationInput) error {
	a.received = input
	return bizerr.NewCode(catalog.CodeManagementNotAccessible)
}

func TestBasicSettingsAtomicPostgreSQL(t *testing.T) {
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
	if _, err = g.DB().Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product ADD COLUMN IF NOT EXISTS sort_order INT DEFAULT 100;
 CREATE TABLE plugin_idc_core_product_directory_membership(tenant_id INT,product_no TEXT PRIMARY KEY,directory_no TEXT);
 CREATE TABLE plugin_idc_core_catalog_product_addon(tenant_id INT,product_no TEXT,addon_product_no TEXT);
 INSERT INTO plugin_idc_core_catalog_product_addon VALUES(0,'product-unified','product-incomplete');`); err != nil {
		t.Fatal(err)
	}
	auth := &basicTestAuthorizer{copyTestAuthorizer: &copyTestAuthorizer{actor: 17}}
	writer, err := catalog.NewManagement(&catalogDraftTestAuditWriter{}, auth)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := catalog.NewPublication(&catalogDraftTestAuditWriter{}, &catalogProductPublicationTestAuthorizer{actorUserID: 17}, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = publication.PublishVersion(ctx, "version-manual-1"); err != nil {
		t.Fatal(err)
	}
	current, err := g.DB().GetValue(ctx, `SELECT version FROM plugin_idc_core_catalog_product WHERE product_no='product-unified'`)
	if err != nil {
		t.Fatal(err)
	}
	empty := []string{}
	input := BasicSettingsInput{Description: catalog.ProductDescriptionUpdateInput{ProductNo: "product-unified", Version: current.Int64(), NameZhCN: "统一修改", NameEnUS: "Updated", SummaryZhCN: "介绍", SummaryEnUS: "Summary"}, VersionNo: "version-copy-2", AddonProductNos: &empty, Inventory: &catalog.ProductVersionInventoryUpdateInput{TrackingEnabled: true, SellableQuantity: 7, ExpectedVersion: 1}}
	assertUnchanged := func() {
		t.Helper()
		row, err := g.DB().GetOne(ctx, `SELECT p.name_en_us,p.version,i.sellable_quantity FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_product_version_inventory i ON i.version_no='version-copy-2' WHERE p.product_no='product-unified'`)
		if err != nil || row["name_en_us"].String() != "Unified product" || row["version"].Int64() != current.Int64() || row["sellable_quantity"].Int64() != 0 {
			t.Fatalf("partial basic settings committed: %v %v", row, err)
		}
		count, err := g.DB().GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_product_addon`)
		if err != nil || count.Int() != 1 {
			t.Fatal("companion set was not rolled back", err)
		}
	}
	input.Inventory.ExpectedVersion = 99
	if _, err = SaveBasicSettings(ctx, input, writer, publication, nil); err == nil {
		t.Fatal("stale stock accepted")
	}
	assertUnchanged()
	input.Inventory.ExpectedVersion = 1
	auth.denyStock = true
	if _, err = SaveBasicSettings(ctx, input, writer, publication, nil); err == nil {
		t.Fatal("stock authority bypassed")
	}
	assertUnchanged()
	auth.denyStock = false
	input.VersionNo = "version-incomplete-1"
	if _, err = SaveBasicSettings(ctx, input, writer, publication, nil); err == nil {
		t.Fatal("foreign version accepted")
	}
	assertUnchanged()
	input.VersionNo = "version-copy-2"
	automation := &basicTestAutomation{}
	hostname := "cloud"
	input.LoginRules = &BasicLoginRulesInput{ExpectedVersion: 3, Values: []productautomation.ConfigurationValue{{FieldKey: "hostname_prefix", TextValue: &hostname}}}
	if _, err = SaveBasicSettings(ctx, input, writer, publication, automation); err == nil {
		t.Fatal("denied configuration accepted")
	}
	assertUnchanged()
	if len(automation.received.CustomerFieldSelections) != 1 || automation.received.CustomerFieldSelections[0].FieldKey != "cpu" || len(automation.received.MerchantValues) != 2 {
		t.Fatal("login form replaced unrelated configuration")
	}
	invalid := "invalid"
	input.LoginRules.Values = []productautomation.ConfigurationValue{{FieldKey: "cpu", TextValue: &invalid}}
	if _, err = SaveBasicSettings(ctx, input, writer, publication, automation); err == nil {
		t.Fatal("non-login parameter accepted")
	}
	assertUnchanged()
	input.LoginRules = nil
	failing, err := catalog.NewPublication(&catalogDraftTestAuditWriter{err: errors.New("audit unavailable")}, &catalogProductPublicationTestAuthorizer{actorUserID: 17}, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SaveBasicSettings(ctx, input, writer, failing, nil); err == nil {
		t.Fatal("failed final audit accepted")
	}
	assertUnchanged()
	saved, err := SaveBasicSettings(ctx, input, writer, publication, nil)
	if err != nil {
		t.Fatal(err)
	}
	row, err := g.DB().GetOne(ctx, `SELECT p.name_en_us,p.version,p.current_version_no,i.sellable_quantity FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_product_version_inventory i ON i.version_no=p.current_version_no WHERE p.product_no='product-unified'`)
	if err != nil || row["name_en_us"].String() != "Updated" || row["sellable_quantity"].Int64() != 7 || row["version"].Int64() != saved || row["current_version_no"].String() != "version-copy-2" {
		t.Fatalf("combined save did not apply: %v %v", row, err)
	}
	count, err := g.DB().GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_product_addon`)
	if err != nil || count.Int() != 0 {
		t.Fatal("empty companion set not persisted", err)
	}
	// Metadata-only staff need no stock or publication authority for untouched sections.
	auth.denyStock = true
	input.Description.Version = saved
	input.Inventory = nil
	input.AddonProductNos = nil
	if _, err = SaveBasicSettings(ctx, input, writer, nil, nil); err != nil {
		t.Fatalf("untouched sections expanded permission: %v", err)
	}
}
