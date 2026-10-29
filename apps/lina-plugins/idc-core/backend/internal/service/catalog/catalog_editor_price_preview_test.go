// This file checks exact checkout price composition and that authorization
// rejects previews before touching persistence.
package catalog

import (
	"context"
	"errors"
	"math"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestProductEditorPreviewAuthorizesBeforePersistence(t *testing.T) {
	denied := errors.New("denied")
	authorizer := &managementTestAuthorizer{err: denied}
	service := &productManagementReaderImpl{authorizer: authorizer}
	if _, err := service.PreviewProductPrice(context.Background(), ProductEditorPricePreviewInput{}); !errors.Is(err, denied) || authorizer.calls.Load() != 1 {
		t.Fatalf("authorization err=%v calls=%d", err, authorizer.calls.Load())
	}
}
func TestProductEditorPreviewSharesCheckoutAmounts(t *testing.T) {
	option := "cpd_11111111111111111111111111111111"
	rate := 1000
	source := &publicQuoteSource{BillingRules: []entity.CatalogBillingRule{{BillingRuleNo: "monthly", ChargeTypeCode: string(ChargeTypeRecurring), PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1, PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModePerUnit), MinimumPurchaseCycles: 1}}, Prices: []entity.CatalogPrice{{BillingRuleNo: "monthly", CurrencyCode: "CNY", AmountMinor: 1000, SetupFeeMinor: 100}, {BillingRuleNo: "monthly", CurrencyCode: "CNY", OptionNo: &option, AmountMinor: 200, SetupFeeMinor: 10}}, TaxPolicies: []entity.CatalogTaxPolicy{{CurrencyCode: "CNY", TaxTreatmentCode: string(TaxTreatmentTaxable), TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service", RoundingScopeCode: string(TaxRoundingScopeTotal), RoundingModeCode: string(TaxRoundingHalfUp), RoundingQuantumMinor: 1}}}
	quote, err := buildPublicConfigurationQuote("", "CNY", source, map[string]int{option: 2})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := productEditorPricePreview("saved-version", quote)
	if err != nil || len(preview.Terms) != 1 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	term := preview.Terms[0]
	if term.BaseAmountMinor != "1000" || term.ConfigurationAmountMinor != "400" || term.SetupAmountMinor != "110" || term.TaxAmountMinor == nil || *term.TaxAmountMinor != "151" || term.TotalAmountMinor == nil || *term.TotalAmountMinor != "1661" {
		t.Fatalf("checkout composition changed: %+v", term)
	}
	quote.Terms[0].Status = PublicQuotePendingUsage
	quote.Terms[0].TotalAmountMinor = nil
	quote.Terms[0].TaxAmountMinor = nil
	preview, err = productEditorPricePreview("saved-version", quote)
	if err != nil || preview.Terms[0].TotalAmountMinor != nil {
		t.Fatalf("invented usage total: %+v err=%v", preview, err)
	}
	quote.Terms[0].Lines = []PublicQuoteLine{{Kind: PublicQuoteLineBase, ChargeAmountMinor: math.MaxInt64}, {Kind: PublicQuoteLineBase, ChargeAmountMinor: 1}}
	if _, err = productEditorPricePreview("saved-version", quote); err == nil {
		t.Fatal("overflow was accepted")
	}
	source.TaxPolicies = nil
	if _, err = buildPublicConfigurationQuote("", "CNY", source, map[string]int{option: 1}); !bizerr.Is(err, CodePublicQuoteNotAvailable) {
		t.Fatal("missing tax was treated as zero tax", err)
	}
}
