// This file exercises formal conversion through the real catalog assembler
// and migrated database, including imported resale rules and setup waiver.
// It verifies pricing only, not conversion checkout or resource recovery.

package renewal

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func TestTrialConversionCatalogResalePostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	// Configure prices while still draft. Publication guards stay enabled.
	if _, err := database.Exec(ctx, genericRenewalProductSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_catalog_price SET setup_fee_minor=2000 WHERE price_no='PRC_renewal_ssl_base';
UPDATE plugin_idc_core_catalog_price SET setup_fee_minor=500 WHERE price_no='PRC_renewal_ssl_wildcard';
UPDATE plugin_idc_core_catalog_tax_policy SET tax_treatment_code='taxable',tax_rate_basis_points=1000;
INSERT INTO plugin_idc_core_catalog_import_term
 (tenant_id,version_no,billing_rule_no,currency_code,upstream_term_reference,whole_item_rule,markup_percent)
VALUES (0,'ver_renewal_ssl_v1','bil_renewal_ssl_month','CNY','upstream-monthly',
 '{"kind":"discount","discountBasisPoints":1000,"prior":{"kind":"fixed","fixedAmountMinor":10001}}','0'),
 (0,'ver_renewal_ssl_v1','bil_renewal_ssl_month','USD','upstream-monthly',
 '{"kind":"fixed","fixedAmountMinor":1}','0');
`+genericRenewalProductPublicationSQL); err != nil {
		t.Fatal(err)
	}
	reader, err := catalog.NewPublicCatalogReaderConfigured(database, []byte("0123456789abcdef0123456789abcdef"), renewalPostgreSQLPurchaseRuntime{}, renewalPostgreSQLFileVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	assembler, err := catalog.RenewalQuoteAssemblerFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	input := catalog.RenewalQuoteInput{CurrencyCode: "CNY", TrialConversion: true, Lines: []catalog.RenewalQuoteLineInput{{
		ReferenceNo: "ENT_conversion", ProductNo: "prd_renewal_ssl", VersionNo: "ver_renewal_ssl_v1",
		BillingRuleNo: "bil_renewal_ssl_month", Quantity: 1, PurchaseCycles: 2,
		PriceSelections: []catalog.ConfigurationPriceSelection{{DimensionNo: "cpd_11111111111111111111111111111111", Quantity: 1}},
	}}}
	lines, err := assembler.AssembleRenewalQuote(ctx, input)
	if err != nil || len(lines) != 1 || len(lines[0].Charges) != 2 {
		t.Fatalf("real conversion catalog: lines=%+v err=%v", lines, err)
	}
	line := lines[0]
	amounts, err := commerce.CalculatePricingAmounts(line, 0)
	// Two periods: 10001*2 less 10% = 18001 before tax; tax rounds to 1800.
	if err != nil || amounts.SetupSubtotalMinor != 0 || amounts.PeriodSubtotalMinor != 18001 ||
		amounts.NetAmountMinor != 18001 || amounts.TaxAmountMinor != 1800 || amounts.TotalAmountMinor != 19801 ||
		line.PriceIncludesTax || line.InventoryDecisionCode != "not_required" {
		t.Fatalf("conversion resale or tax incorrect: amounts=%+v line=%+v err=%v", amounts, line, err)
	}
	for _, charge := range line.Charges {
		if charge.SetupFeeMinor != 0 || charge.PricedAmountMinor != charge.ChargeAmountMinor {
			t.Fatalf("resale restored an installation fee: %+v", charge)
		}
	}
	setup, err := database.GetValue(ctx, `SELECT SUM(setup_fee_minor) FROM plugin_idc_core_catalog_price`)
	if err != nil || setup.Int64() != 2500 {
		t.Fatalf("conversion changed source sale prices: setup=%v err=%v", setup, err)
	}
	// A later sale stop must reject a new quote even though the same immutable
	// formal period and resale price still exist in the database.
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='prd_renewal_ssl'`); err != nil {
		t.Fatal(err)
	}
	if lines, err := assembler.AssembleRenewalQuote(ctx, input); !bizerr.Is(err, catalog.CodePublicQuoteNotAvailable) || lines != nil {
		t.Fatalf("stopped product accepted conversion: lines=%+v err=%v", lines, err)
	}
}
