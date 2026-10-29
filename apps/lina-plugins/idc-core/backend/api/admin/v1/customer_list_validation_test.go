//go:build legacy_server_catalog_tests

package v1

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestCustomerListRequestAcceptsExactBusinessNumberLength(t *testing.T) {
	req := &CustomerListReq{
		GroupNo: "GRP_11111111111111111111111111111111",
		TagNo:   "TAG_22222222222222222222222222222222",
		Limit:   25,
	}
	if err := g.Validator().Data(req).Run(context.Background()); err != nil {
		t.Fatalf("validate exact-length customer segments: %v", err)
	}

	req.GroupNo += "1"
	if err := g.Validator().Data(req).Run(context.Background()); err == nil {
		t.Fatal("overlong customer group business number was accepted")
	}
}

func TestCustomerCursorRequestLengthIsBounded(t *testing.T) {
	if err := g.Validator().Data(&CustomerListReq{
		Cursor: strings.Repeat("a", 1024), Limit: 25,
	}).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum-length customer cursor: %v", err)
	}
	if err := g.Validator().Data(&CustomerSegmentListReq{
		Kind: "tag", Locale: "zh-CN", Limit: 25,
		Cursor: strings.Repeat("a", 1025),
	}).Run(context.Background()); err == nil {
		t.Fatal("overlong customer segment cursor was accepted")
	}
}

func TestCatalogProductListRequestIsBounded(t *testing.T) {
	if err := g.Validator().Data(&CatalogProductListReq{
		Cursor: strings.Repeat("a", 128), Limit: 100,
	}).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum catalog product list request: %v", err)
	}
	if err := g.Validator().Data(&CatalogProductListReq{
		Cursor: strings.Repeat("a", 129), Limit: 101,
	}).Run(context.Background()); err == nil {
		t.Fatal("overlong catalog product list request was accepted")
	}
}

func TestCatalogProductPublishVersionNumberIsBounded(t *testing.T) {
	if err := g.Validator().Data(&CatalogProductPublishReq{
		VersionNo: strings.Repeat("a", 128),
	}).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum catalog product publication version number: %v", err)
	}
	if err := g.Validator().Data(&CatalogProductPublishReq{
		VersionNo: strings.Repeat("a", 129),
	}).Run(context.Background()); err == nil {
		t.Fatal("overlong catalog product publication version number was accepted")
	}
}

func TestCatalogProductPublicationPreviewVersionNumberIsBounded(t *testing.T) {
	if err := g.Validator().Data(&CatalogProductPublicationPreviewReq{
		VersionNo: strings.Repeat("a", 128),
	}).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum catalog publication preview version number: %v", err)
	}
	if err := g.Validator().Data(&CatalogProductPublicationPreviewReq{
		VersionNo: strings.Repeat("a", 129),
	}).Run(context.Background()); err == nil {
		t.Fatal("overlong catalog publication preview version number was accepted")
	}
}

func TestCatalogCategoryAndProductCreateRequestsAreBounded(t *testing.T) {
	category := &CatalogCategoryCreateReq{
		NameZhCn: strings.Repeat("证", 128),
		NameEnUs: strings.Repeat("a", 128), SortOrder: 1_000_000,
	}
	if err := g.Validator().Data(category).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum category create request: %v", err)
	}
	category.NameEnUs += "a"
	if err := g.Validator().Data(category).Run(context.Background()); err == nil {
		t.Fatal("overlong category name was accepted")
	}
	product := &CatalogProductCreateReq{
		CategoryNo: "ctg_0123456789abcdef0123456789abcdef", PublicSlug: "managed-ssl",
		NameZhCn: "托管 SSL 证书", NameEnUs: "Managed SSL",
		SummaryZhCn: strings.Repeat("云", 1024), SortOrder: -1_000_000,
	}
	if err := g.Validator().Data(product).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum product create request: %v", err)
	}
	product.SummaryZhCn += "云"
	if err := g.Validator().Data(product).Run(context.Background()); err == nil {
		t.Fatal("overlong product summary was accepted")
	}
}

func TestCatalogBillingAndPriceCreateRequestsAreBounded(t *testing.T) {
	billing := &CatalogBillingRuleCreateReq{
		VersionNo: "version-draft-1", RuleCode: strings.Repeat("a", 64),
		ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1,
		PricePhase: "both", QuantityMode: "fixed", MinimumPurchaseCycles: 1,
		Status: "active",
	}
	if err := g.Validator().Data(billing).Run(context.Background()); err != nil {
		t.Fatalf("validate maximum billing-rule request: %v", err)
	}
	billing.RuleCode += "a"
	if err := g.Validator().Data(billing).Run(context.Background()); err == nil {
		t.Fatal("overlong billing-rule code was accepted")
	}
	renewal := int64(10_900)
	price := &CatalogPriceScheduleCreateReq{
		VersionNo: "version-draft-1", BillingRuleNo: "billing-monthly",
		CurrencyCode: "CNY", Status: "active", Tiers: []CatalogPriceTierInput{{
			MinimumQuantity: 0, AmountMinor: 9_900, RenewalAmountMinor: &renewal,
		}},
	}
	if err := g.Validator().Data(price).Run(context.Background()); err != nil {
		t.Fatalf("validate price-schedule request: %v", err)
	}
	price.CurrencyCode = "CN"
	if err := g.Validator().Data(price).Run(context.Background()); err == nil {
		t.Fatal("short currency code was accepted")
	}
	price.CurrencyCode = "CNY"
	price.Tiers = nil
	if err := g.Validator().Data(price).Run(context.Background()); err == nil {
		t.Fatal("empty price schedule was accepted")
	}
}
