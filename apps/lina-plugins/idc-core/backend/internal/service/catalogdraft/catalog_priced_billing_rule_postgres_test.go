// This file proves a normal period and its first price become visible together,
// including permission/price/audit rollback and immutable historical amounts.
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
)

func TestPricedBillingRuleAtomicPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	installAutomationPublicationMigration(t, ctx)
	insertUnifiedPublicationCatalog(t, ctx)
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/081-simple-product-version-inventory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	authority := &catalogProductPublicationTestAuthorizer{actorUserID: 17}
	publication, err := catalog.NewPublication(&catalogDraftTestAuditWriter{}, authority, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = publication.PublishVersion(ctx, "version-manual-1"); err != nil {
		t.Fatal(err)
	}
	access := &pricedPeriodTestAuthorizer{}
	manager, err := catalog.NewManagement(&catalogDraftTestAuditWriter{}, access)
	if err != nil {
		t.Fatal(err)
	}
	renewal := int64(25000)
	input := catalog.PricedBillingRuleCreateInput{VersionNo: "version-copy-2", RuleCode: "quarterly", PeriodUnit: catalog.PeriodUnitMonth, PeriodCount: 3, MinimumPurchaseCycles: 1, CurrencyCode: "CNY", Price: catalog.PriceTierDefinition{MinimumQuantity: 0, AmountMinor: 23000, SetupFeeMinor: 100, RenewalAmountMinor: &renewal}}
	var originalRules, originalPrices int
	v, e := g.DB().GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_billing_rule WHERE version_no=?`, input.VersionNo)
	if e != nil {
		t.Fatal(e)
	}
	originalRules = v.Int()
	v, e = g.DB().GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_price WHERE version_no=?`, input.VersionNo)
	if e != nil {
		t.Fatal(e)
	}
	originalPrices = v.Int()
	assertUnchanged := func() {
		t.Helper()
		for table, want := range map[string]int{"plugin_idc_core_catalog_billing_rule": originalRules, "plugin_idc_core_catalog_price": originalPrices} {
			got, e := g.DB().GetValue(ctx, "SELECT COUNT(*) FROM "+table+" WHERE version_no=?", input.VersionNo)
			if e != nil || got.Int() != want {
				t.Fatalf("partial priced period in %s: %v %v", table, got, e)
			}
		}
		assertUnifiedPublicationState(t, ctx, "published", "draft", "version-manual-1")
	}
	// A valid billing definition must not survive a missing renewal price.
	input.Price.RenewalAmountMinor = nil
	if _, err = catalog.CreatePricedBillingRule(ctx, manager, publication, input); err == nil {
		t.Fatal("missing renewal price accepted")
	}
	assertUnchanged()
	input.Price.RenewalAmountMinor = &renewal
	// Billing permission alone does not permit the bundled price write.
	access.denyPrice = true
	if _, err = catalog.CreatePricedBillingRule(ctx, manager, publication, input); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("price permission bypassed: %v", err)
	}
	assertUnchanged()
	access.denyPrice = false
	failing, err := catalog.NewPublication(&catalogDraftTestAuditWriter{err: errors.New("injected final save audit failure")}, authority, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.CreatePricedBillingRule(ctx, manager, failing, input); err == nil {
		t.Fatal("final save audit failure accepted")
	}
	assertUnchanged()
	rule, err := catalog.CreatePricedBillingRule(ctx, manager, publication, input)
	if err != nil {
		t.Fatal(err)
	}
	row, err := g.DB().GetOne(ctx, `SELECT r.period_unit_code,r.period_count,p.amount_minor,p.setup_fee_minor,p.renewal_amount_minor,v.status_code FROM plugin_idc_core_catalog_billing_rule r JOIN plugin_idc_core_catalog_price p USING(version_no,billing_rule_no) JOIN plugin_idc_core_catalog_product_version v USING(version_no) WHERE r.billing_rule_no=?`, rule.BillingRuleNo)
	if err != nil || row["period_unit_code"].String() != "month" || row["period_count"].Int() != 3 || row["amount_minor"].Int64() != 23000 || row["setup_fee_minor"].Int64() != 100 || row["renewal_amount_minor"].Int64() != 25000 || row["status_code"].String() != "published" {
		t.Fatalf("period and price did not apply together: %v %v", row, err)
	}
	historical, err := g.DB().GetValue(ctx, `SELECT amount_minor FROM plugin_idc_core_catalog_price WHERE price_no='price-manual-base'`)
	if err != nil || historical.Int64() != 10000 {
		t.Fatalf("historical amount changed: %v %v", historical, err)
	}
	if _, err = catalog.CreatePricedBillingRule(ctx, manager, publication, input); err == nil {
		t.Fatal("published target accepted a response-lost duplicate")
	}
	// A one-time free price is explicit and never acquires a guessed renewal fee.
	// This shared fixture labels its incomplete product as a legacy migration;
	// model an ordinary editable manual product for this independent scenario.
	if _, err = g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET source_code='manual' WHERE version_no='version-incomplete-1'`); err != nil {
		t.Fatal(err)
	}
	free := catalog.PricedBillingRuleCreateInput{VersionNo: "version-incomplete-1", RuleCode: "once", PeriodUnit: catalog.PeriodUnitOnce, PeriodCount: 1, MinimumPurchaseCycles: 1, CurrencyCode: "CNY", Price: catalog.PriceTierDefinition{MinimumQuantity: 0, AmountMinor: 0}}
	oneTime, err := catalog.CreatePricedBillingRule(ctx, manager, publication, free)
	if err != nil {
		t.Fatal(err)
	}
	row, err = g.DB().GetOne(ctx, `SELECT r.charge_type_code,r.price_phase_code,p.amount_minor,p.renewal_amount_minor FROM plugin_idc_core_catalog_billing_rule r JOIN plugin_idc_core_catalog_price p USING(version_no,billing_rule_no) WHERE r.billing_rule_no=?`, oneTime.BillingRuleNo)
	if err != nil || row["charge_type_code"].String() != "one_time" || row["price_phase_code"].String() != "new_purchase" || row["amount_minor"].Int64() != 0 || !row["renewal_amount_minor"].IsNil() {
		t.Fatalf("invalid free one-time period: %v %v", row, err)
	}
}

type pricedPeriodTestAuthorizer struct {
	catalog.ManagementAuthorizer
	denyPrice bool
}

func (*pricedPeriodTestAuthorizer) AuthorizeBillingWrite(context.Context, string) (int, error) {
	return 17, nil
}
func (a *pricedPeriodTestAuthorizer) AuthorizePriceWrite(context.Context, string) (int, error) {
	if a.denyPrice {
		return 0, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return 17, nil
}
