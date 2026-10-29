// Merchant pricing changes retain the supplier's immutable contract and exact
// period/dimension identities; rejected edits roll back the entire command.
package catalog

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func TestImportedPricingOnlyUpdatePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, catalogImportMigrationFixtureSQL+catalogImportPersistenceFixtureSQL+portableImportFixtureSQL+`CREATE TABLE plugin_idc_core_commerce_configuration_snapshot(id BIGSERIAL);`); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("migration path unavailable")
	}
	for _, name := range []string{"110-catalog-product-import.sql", "120-portable-catalog-import.sql", "153-native-import-purchase-pricing.sql", "153-native-import-purchase-pricing.sql"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := newManagementService(&managementTestAudit{}, &managementTestAuthorizer{actor: 7}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	source := portableImportTestSource()
	input := ImportInput{RequestKey: "saved-native-policy-initial", InterfaceInstanceNo: source.ReadEvidence.InterfaceInstanceNo, Reference: "demo", CategoryNo: "ctg_" + strings.Repeat("1", 32), PricingMode: ImportPricingMarkup, MarkupPercent: "10"}
	commit := func(in ImportInput, detail *resourceinterfaceprojectioncap.CatalogDetail) (*ImportResult, error) {
		var result *ImportResult
		err := db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			var e error
			result, _, e = svc.CommitImport(ctx, in, detail, nil)
			return e
		})
		return result, err
	}
	first, err := commit(input, source)
	if err != nil {
		t.Fatal(err)
	}
	input.TargetVersionNo = first.Item.VersionNo
	input.RequestKey = "saved-native-policy-markup25"
	input.MarkupPercent = "25"
	before, err := db.GetValue(ctx, `SELECT snapshot_digest FROM plugin_idc_core_catalog_import_contract WHERE version_no=?`, first.Item.VersionNo)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := db.GetValue(ctx, `SELECT string_agg(billing_rule_no||':'||currency_code,',' ORDER BY billing_rule_no) FROM plugin_idc_core_catalog_import_term WHERE version_no=?`, first.Item.VersionNo)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := commit(input, source)
	if err != nil || !changed.PricingOnly || changed.Item.VersionNo != first.Item.VersionNo {
		t.Fatalf("pricing-only update %+v err %v", changed, err)
	}
	replay, replayErr := svc.LookupImport(ctx, input)
	if replayErr != nil || replay == nil || !replay.PricingOnly || replay.Item.VersionNo != first.Item.VersionNo {
		t.Fatalf("price command replay %+v %v", replay, replayErr)
	}
	changedBody := input
	changedBody.MarkupPercent = "26"
	if _, replayErr = svc.LookupImport(ctx, changedBody); !bizerr.Is(replayErr, CodeManagementConflict) {
		t.Fatal("price command key accepted changed body")
	}
	sourceRecords, countErr := db.GetCount(ctx, "SELECT 1 FROM plugin_idc_core_catalog_product_import")
	if countErr != nil || sourceRecords != 1 {
		t.Fatal("price save created another source import")
	}
	policy, err := ReadImportedNativePricing(ctx, db, first.Item.VersionNo)
	if err != nil || policy.Mode != "markup" || policy.MarkupPercent != "25" {
		t.Fatalf("saved markup policy %+v %v", policy, err)
	}
	fixed, err := db.GetValue(ctx, `SELECT whole_item_rule->>'fixedAmountMinor' FROM plugin_idc_core_catalog_import_term WHERE version_no=?`, first.Item.VersionNo)
	if err != nil || fixed.Int64() != 250 {
		t.Fatalf("markup did not change actual whole price: %v %v", fixed, err)
	}
	after, err := db.GetValue(ctx, `SELECT snapshot_digest FROM plugin_idc_core_catalog_import_contract WHERE version_no=?`, first.Item.VersionNo)
	if err != nil || after.String() != before.String() {
		t.Fatal("pricing update changed source snapshot")
	}
	afterIdentity, err := db.GetValue(ctx, `SELECT string_agg(billing_rule_no||':'||currency_code,',' ORDER BY billing_rule_no) FROM plugin_idc_core_catalog_import_term WHERE version_no=?`, first.Item.VersionNo)
	if err != nil || afterIdentity.String() != identity.String() {
		t.Fatal("pricing update replaced source periods")
	}
	input.RequestKey = "saved-native-policy-manual"
	input.PricingMode = ImportPricingManual
	input.MarkupPercent = ""
	input.Prices = []resourceinterfaceprojectioncap.CatalogPrice{{CurrencyCode: "USD", Period: resourceinterfaceprojectioncap.CatalogPeriodMonth, PeriodCount: 1, Amount: "9.99", SetupFee: "0.40"}}
	if _, err = commit(input, source); err != nil {
		t.Fatal(err)
	}
	policy, err = ReadImportedNativePricing(ctx, db, first.Item.VersionNo)
	if err != nil || policy.Mode != "manual" || len(policy.ManualPrices) != 1 || policy.ManualPrices[0].Amount != "9.99" || policy.ManualPrices[0].SetupFee != "0.40" {
		t.Fatalf("saved manual policy %+v %v", policy, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_price SET status_code='inactive' WHERE version_no=?`, first.Item.VersionNo); err != nil {
		t.Fatal(err)
	}
	if policy, err = ReadImportedNativePricing(ctx, db, first.Item.VersionNo); err != nil || len(policy.ManualPrices) != 1 {
		t.Fatalf("inactive manual price cannot be restored %+v %v", policy, err)
	}
	changedSource := *source
	changedSnapshot := *source.ImportSnapshot
	changedSnapshot.ConfigurationRevision = "changed-configuration"
	changedSource.ImportSnapshot = &changedSnapshot
	input.RequestKey = "saved-native-policy-refuse-source"
	if _, err = commit(input, &changedSource); !bizerr.Is(err, CodeUpstreamConfigurationLocked) {
		t.Fatalf("changed source admitted %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_import_term SET upstream_term_reference='other' WHERE version_no=?`, first.Item.VersionNo); err == nil {
		t.Fatal("SQL allowed source identity change")
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?`, first.Item.VersionNo); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_import_term SET markup_percent='99' WHERE version_no=?`, first.Item.VersionNo); err == nil {
		t.Fatal("SQL changed published historical price")
	}
	var sourceContract struct{ Snapshot json.RawMessage }
	if err = db.GetScan(ctx, &sourceContract, `SELECT snapshot FROM plugin_idc_core_catalog_import_contract WHERE version_no=?`, first.Item.VersionNo); err != nil || len(sourceContract.Snapshot) == 0 {
		t.Fatal("source evidence missing")
	}
}
