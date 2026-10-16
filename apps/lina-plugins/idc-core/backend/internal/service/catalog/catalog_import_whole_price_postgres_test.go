// This file verifies persisted resale rules, currency isolation, re-export and
// public/purchase quote agreement using an isolated PostgreSQL schema.

package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestImportedWholePriceExportPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_import_term(tenant_id INT,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,whole_item_rule JSONB)`); err != nil {
		t.Fatal(err)
	}
	fixed, usdFixed, bps, rate := int64(10001), int64(300), 1000, 1000
	rule := channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &bps, Prior: &channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed}}
	for currency, value := range map[string]channelcommercecap.ImportWholeItemRule{"CNY": rule, "USD": {Kind: "fixed", FixedAmountMinor: &usdFixed}} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Model(do.CatalogImportTerm{}).Ctx(ctx).Data(do.CatalogImportTerm{TenantId: 0, VersionNo: "v1", BillingRuleNo: "monthly", CurrencyCode: currency, WholeItemRule: string(raw)}).Insert(); err != nil {
			t.Fatal(err)
		}
	}
	reader := &publicCatalogReaderImpl{database: db}
	rules, err := reader.importedWholePrices(ctx, []string{"v1"}, "")
	if err != nil || len(rules) != 2 {
		t.Fatalf("all-currency source: %+v %v", rules, err)
	}
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &publicStructuredConfigurationSnapshot{Target: publicStructuredConfigurationTarget{VersionNo: "v1"}, Revision: "r1", Schema: &capmodel.ConfigurationSchemaDeclaration{}}
	projection := &importSchemaProjection{schema: snapshot.Schema, fields: map[string]string{}, codec: codec, revision: snapshot.Revision}
	source := &publicQuoteSource{BillingRules: []entity.CatalogBillingRule{{BillingRuleNo: "monthly", VersionNo: "v1", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, PricePhaseCode: "new_purchase", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1}}}
	for _, currency := range []string{"CNY", "USD"} {
		source.Prices = append(source.Prices, entity.CatalogPrice{VersionNo: "v1", BillingRuleNo: "monthly", CurrencyCode: currency, AmountMinor: 11000, SetupFeeMinor: 1100})
		source.TaxPolicies = append(source.TaxPolicies, entity.CatalogTaxPolicy{VersionNo: "v1", CurrencyCode: currency, PriceIncludesTax: true, TaxTreatmentCode: "taxable", TaxRateBasisPoints: &rate, TaxCategoryCode: "service", RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1})
	}
	exported, err := projectImportPrices(codec, "demo", snapshot, projection, source, nil, rules)
	if err != nil || len(exported) != 2 {
		t.Fatalf("export: %+v %v", exported, err)
	}
	for _, price := range exported {
		if price.WholeItemRule == nil {
			t.Fatal("lost imported whole-item price")
		}
		if price.CurrencyCode == "CNY" && (price.WholeItemRule.Prior == nil || *price.WholeItemRule.Prior.FixedAmountMinor != fixed) {
			t.Fatal("lost prior fixed price")
		}
		if price.CurrencyCode == "USD" && (price.WholeItemRule.Prior != nil || *price.WholeItemRule.FixedAmountMinor != usdFixed) {
			t.Fatal("cross-currency rule applied")
		}
	}
	line := PurchaseQuoteLine{VersionNo: "v1", BillingRuleNo: "monthly", Quantity: 1, PurchaseCycles: 1, PriceIncludesTax: true, TaxTreatmentCode: "taxable", TaxRateBasisPoints: &rate, RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1, Charges: []PurchaseQuoteCharge{{ChargeAmountMinor: 11000, SetupFeeMinor: 1100, PricedAmountMinor: 12100}}}
	purchase, err := reader.applyImportedPurchasePrices(ctx, "CNY", []string{"v1"}, []PurchaseQuoteLine{line})
	if err != nil || purchase[0].PriceIncludesTax || purchase[0].Charges[0].ChargeAmountMinor != 9000 || purchase[0].Charges[0].SetupFeeMinor != 1000 {
		t.Fatalf("persisted purchase amount: %+v %v", purchase, err)
	}
	originalNet, originalTax, originalTotal := int64(11000), int64(1100), int64(12100)
	quote := &PublicConfigurationQuote{CurrencyCode: "CNY", Tax: PublicQuoteTaxSnapshot{PriceIncludesTax: true, TaxTreatment: TaxTreatmentTaxable, TaxRateBasisPoints: &rate, TaxCategoryCode: "service", RoundingScope: TaxRoundingScopeLine, RoundingMode: TaxRoundingHalfUp, RoundingQuantumMinor: 1}}
	for _, termNo := range []string{"monthly", "annual"} {
		quote.Terms = append(quote.Terms, PublicQuoteTerm{TermNo: termNo, Status: PublicQuoteFinal, Cycles: 1, Lines: []PublicQuoteLine{{ChargeAmountMinor: 11000, SetupFeeMinor: 1100, PricedAmountMinor: 12100}}, NetAmountMinor: &originalNet, TaxAmountMinor: &originalTax, TotalAmountMinor: &originalTotal})
	}
	if err = reader.applyImportedPublicPrices(ctx, "v1", quote); err != nil {
		t.Fatal(err)
	}
	if quote.Tax.PriceIncludesTax || *quote.Terms[0].NetAmountMinor != 10000 || *quote.Terms[0].TaxAmountMinor != 1000 || *quote.Terms[0].TotalAmountMinor != 11000 || quote.Terms[0].Lines[0].PricedAmountMinor != purchase[0].Charges[0].PricedAmountMinor {
		t.Fatalf("public/purchase mismatch: %+v %+v", quote, quote.Terms[0])
	}
	if quote.Terms[1].Lines[0].PricedAmountMinor != 11000 || *quote.Terms[1].TotalAmountMinor != 12100 || *quote.Terms[1].TaxAmountMinor != 1100 {
		t.Fatalf("unadjusted term mixed tax units: %+v", quote.Terms[1])
	}
}
