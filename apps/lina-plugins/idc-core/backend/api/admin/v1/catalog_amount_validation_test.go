package v1

import (
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestCatalogPriceScheduleAcceptsExactInt64Amounts(t *testing.T) {
	t.Parallel()
	maximum := "9223372036854775807"
	request := &CatalogPriceScheduleCreateReq{
		VersionNo:     "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BillingRuleNo: "bil_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CurrencyCode:  "CNY", Status: "active",
		Tiers: []CatalogPriceTierInput{{
			MinimumQuantity: 0, AmountMinor: maximum, SetupFeeMinor: "0",
			RenewalAmountMinor: &maximum,
		}},
	}
	if err := g.Validator().Data(request).Run(t.Context()); err != nil {
		t.Fatalf("valid exact Int64 amount rejected: %v", err)
	}
}

func TestCatalogPriceScheduleRejectsOverlongAmount(t *testing.T) {
	t.Parallel()
	request := &CatalogPriceScheduleCreateReq{
		VersionNo:     "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BillingRuleNo: "bil_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CurrencyCode:  "CNY", Status: "active",
		Tiers: []CatalogPriceTierInput{{
			MinimumQuantity: 0, AmountMinor: "92233720368547758070", SetupFeeMinor: "0",
		}},
	}
	if err := g.Validator().Data(request).Run(t.Context()); err == nil {
		t.Fatal("overlong amount was accepted")
	}
}

func TestCatalogProductListCategoryNumberValidation(t *testing.T) {
	t.Parallel()
	for _, categoryNo := range []string{"", "ctg_0123456789abcdef0123456789abcdef"} {
		request := &CatalogProductListReq{CategoryNo: categoryNo, Limit: 25}
		if err := g.Validator().Data(request).Run(t.Context()); err != nil {
			t.Fatalf("valid category number %q rejected: %v", categoryNo, err)
		}
	}

	request := &CatalogProductListReq{CategoryNo: "ctg_cloud", Limit: 25}
	if err := g.Validator().Data(request).Run(t.Context()); err == nil {
		t.Fatal("legacy category number was accepted")
	}
}
