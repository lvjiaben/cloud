// This file checks bounded sales projections and preserves the distinction
// between missing prices, free prices, base prices and option surcharges.
package resourceinterfaceprojectioncap

import (
	"strings"
	"testing"
)

func TestCatalogPageRequiresExactVisibleCount(t *testing.T) {
	t.Parallel()
	query := CatalogQuery{Page: 2, PageSize: 10}
	page := CatalogPage{Total: 11, Items: []CatalogProduct{{Reference: "101", Name: "Cloud server"}}}
	if err := page.Validate(query); err != nil {
		t.Fatal(err)
	}
	page.Total = 12
	if page.Validate(query) == nil {
		t.Fatal("incomplete supplier snapshot was accepted")
	}
	page = CatalogPage{Total: 0, Items: nil}
	if err := page.Validate(CatalogQuery{Page: 1, PageSize: 10}); err != nil {
		t.Fatal(err)
	}
	if page.Validate(CatalogQuery{Page: 0, PageSize: 10}) == nil {
		t.Fatal("invalid page was accepted")
	}
	if page.Validate(CatalogQuery{Page: 1, PageSize: 101}) == nil {
		t.Fatal("unbounded page was accepted")
	}
}

func TestCatalogDetailPreservesPricesWithoutCurrencyInference(t *testing.T) {
	t.Parallel()
	query := CatalogProductQuery{Reference: "101"}
	price := CatalogPrice{CurrencyCode: "CNY", Period: CatalogPeriodMonth, PeriodCount: 1, Amount: "100.005", SetupFee: "0"}
	detail := CatalogDetail{
		Product: CatalogProduct{Reference: "101", Name: "服务器"}, BasePrices: []CatalogPrice{price},
		Options: []CatalogOption{{Reference: "10", Name: "自定义配置", Choices: []CatalogChoice{{Reference: "20", Name: "基础选项", Prices: []CatalogPrice{price}}}}},
	}
	if err := detail.Validate(query); err != nil {
		t.Fatal(err)
	}
	if detail.BasePrices[0].Amount != "100.005" || detail.Options[0].Choices[0].Prices[0].Amount != "100.005" {
		t.Fatal("component precision was changed")
	}
	for _, amount := range []string{"-1", "", "NaN", "1e3", "0.01secret", strings.Repeat("1", 25)} {
		invalid := detail
		invalid.BasePrices = []CatalogPrice{price}
		invalid.BasePrices[0].Amount = amount
		if invalid.Validate(query) == nil {
			t.Fatalf("invalid money %q accepted", amount)
		}
	}
	detail.BasePrices = nil
	if err := detail.Validate(query); err != nil {
		t.Fatalf("missing schedule must stay distinguishable from zero: %v", err)
	}
}

func TestCatalogDetailRejectsWrongProductDuplicateAndUnprovenPeriod(t *testing.T) {
	t.Parallel()
	query := CatalogProductQuery{Reference: "101"}
	base := CatalogDetail{Product: CatalogProduct{Reference: "101", Name: "服务器"}}
	tests := []struct {
		name   string
		mutate func(*CatalogDetail)
	}{
		{"wrong target", func(d *CatalogDetail) { d.Product.Reference = "102" }},
		{"control text", func(d *CatalogDetail) { d.Product.Name = "server\x00" }},
		{"duplicate options", func(d *CatalogDetail) {
			d.Options = []CatalogOption{{Reference: "10", Name: "A"}, {Reference: "10", Name: "B"}}
		}},
		{"unproven period", func(d *CatalogDetail) {
			d.BasePrices = []CatalogPrice{{CurrencyCode: "CNY", Period: "每月", PeriodCount: 1, Amount: "1", SetupFee: "0"}}
		}},
		{"missing fee", func(d *CatalogDetail) {
			d.BasePrices = []CatalogPrice{{CurrencyCode: "CNY", Period: CatalogPeriodMonth, PeriodCount: 1, Amount: "1"}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := base
			test.mutate(&value)
			if value.Validate(query) == nil {
				t.Fatal("invalid detail accepted")
			}
		})
	}
}
