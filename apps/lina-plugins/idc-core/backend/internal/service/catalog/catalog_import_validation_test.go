// This file verifies import pricing eligibility, ISO precision, and bounded
// request validation without supplier calls or database dependencies.

package catalog

import (
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"strings"
	"testing"
)

func TestImportCurrencyMinorUnitAndHalfUp(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		code, amount string
		precision    int
		want         int64
	}{{"CNY", "0.005", 2, 1}, {"USD", "1.005", 2, 101}, {"JPY", "0.5", 0, 1}, {"KWD", "0.0005", 3, 1}, {"BHD", "1.2345", 3, 1235}} {
		precision, err := importCurrencyMinorUnit(test.code)
		if err != nil || precision != test.precision {
			t.Fatalf("%s precision=%d err=%v", test.code, precision, err)
		}
		got, err := importFinalPriceMinor(test.amount, precision)
		if err != nil || got != test.want {
			t.Fatalf("%s price=%d err=%v", test.code, got, err)
		}
	}
	for _, code := range []string{"ZZZ", "XXX", "XAU", "XDR", "usd", " USD", ""} {
		if _, err := importCurrencyMinorUnit(code); !bizerr.Is(err, CodeManagementInvalid) {
			t.Fatalf("accepted non-currency %q: %v", code, err)
		}
	}
}

func TestPrepareImportSourceNeverPricesAnIncompleteMachine(t *testing.T) {
	t.Parallel()
	price := resourceinterfaceprojectioncap.CatalogPrice{CurrencyCode: "CNY", Period: resourceinterfaceprojectioncap.CatalogPeriodMonth, PeriodCount: 1, Amount: "100", SetupFee: "0"}
	source := &resourceinterfaceprojectioncap.CatalogDetail{Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "101", Name: "Upstream"}, BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{price}, PriceScheduleComplete: true}
	input := ImportInput{Reference: "101", PricingMode: ImportPricingMarkup, MarkupPercent: "10"}
	_, prices, incomplete, err := prepareImportSource(input, source)
	if err != nil || incomplete || len(prices) != 1 || prices[0].amountMinor != 11000 {
		t.Fatalf("complete uplift=%+v incomplete=%v err=%v", prices, incomplete, err)
	}
	if prices[0].definition.PricePhase != PricePhaseNewPurchase {
		t.Fatal("import inferred renewal pricing")
	}
	for _, hasOptions := range []bool{false, true} {
		copySource := *source
		copySource.PriceScheduleComplete = false
		if hasOptions {
			copySource.PriceScheduleComplete = true
			copySource.Options = []resourceinterfaceprojectioncap.CatalogOption{{Reference: "cpu", Name: "Upstream configuration"}}
		}
		encoded, got, missing, priceErr := prepareImportSource(input, &copySource)
		if priceErr != nil || !missing || len(got) != 0 || !strings.Contains(string(encoded), `"productConfiguration":[]`) {
			t.Fatalf("incomplete source created prices: %s %+v %v %v", encoded, got, missing, priceErr)
		}
	}
	input.PricingMode = ImportPricingManual
	input.MarkupPercent = ""
	price.Amount = "73.005"
	input.Prices = []resourceinterfaceprojectioncap.CatalogPrice{price}
	source.PriceScheduleComplete = false
	_, prices, incomplete, err = prepareImportSource(input, source)
	if err != nil || incomplete || len(prices) != 1 || prices[0].amountMinor != 7301 {
		t.Fatalf("manual final price=%+v incomplete=%v err=%v", prices, incomplete, err)
	}
}

func TestImportInputBoundsAndDigest(t *testing.T) {
	t.Parallel()
	valid := ImportInput{RequestKey: "test-request-000001", InterfaceInstanceNo: "pfi_" + strings.Repeat("a", 32), Reference: "101", CategoryNo: "ctg_" + strings.Repeat("b", 32), PricingMode: ImportPricingMarkup, MarkupPercent: "10"}
	digest, err := validateImportInput(valid)
	if err != nil || len(digest) != 64 {
		t.Fatal(digest, err)
	}
	for _, mutate := range []func(*ImportInput){func(v *ImportInput) { v.MarkupPercent = "NaN" }, func(v *ImportInput) { v.MarkupPercent = "-1" }, func(v *ImportInput) { v.RequestKey = "short" }, func(v *ImportInput) { v.Prices = make([]resourceinterfaceprojectioncap.CatalogPrice, 121) }, func(v *ImportInput) { v.PricingMode = ImportPricingManual; v.MarkupPercent = "" }} {
		input := valid
		mutate(&input)
		if _, validationErr := validateImportInput(input); !bizerr.Is(validationErr, CodeManagementInvalid) {
			t.Fatalf("accepted invalid input %+v: %v", input, validationErr)
		}
	}
	changed := valid
	changed.MarkupPercent = "11"
	other, err := validateImportInput(changed)
	if err != nil || other == digest {
		t.Fatal("changed request digest unchanged")
	}
}
