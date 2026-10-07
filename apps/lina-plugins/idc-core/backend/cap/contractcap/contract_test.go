package contractcap

import "testing"

func TestBuildPreviewVariablesKeepsLinesAndAuthoritativeDateBlank(t *testing.T) {
	snapshot := &OrderSnapshot{
		OrderNo: "ord_contract_preview_1", CurrencyCode: "CNY", AmountMinor: 36_800,
		RequiresContract: true, OrderVersion: 3,
		Template: TemplateSnapshot{
			TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 7,
			Name: "服务器服务合同", LanguageCode: "zh-CN", ScopeCode: "all_customers",
			SigningOrderCode: "customer_first",
		},
		Merchant: MerchantIdentity{
			MerchantNo: "merchant-1", LegalName: []byte("示例服务商"),
			CountryCode: "CN", Version: 2,
		},
		Customer: VerifiedIdentity{LegalName: []byte("示例客户")},
		Items: []OrderItem{
			{
				OrderItemNo: "item-1", ContractProductName: "香港云服务器",
				Quantity: 2, PurchaseCycles: 12, PeriodUnitCode: "month",
				PeriodCount: 1, AmountMinor: 24_000,
			},
			{
				OrderItemNo: "item-2", ContractProductName: "备份服务",
				Quantity: 1, PurchaseCycles: 12, PeriodUnitCode: "month",
				PeriodCount: 1, AmountMinor: 12_800,
			},
		},
	}
	preview, err := snapshot.BuildPreviewVariables("contract-preview-1")
	if err != nil {
		t.Fatalf("build preview variables: %v", err)
	}
	defer preview.Clear()
	if len(preview.Items) != 2 || preview.Items[0].ServiceName != "香港云服务器" ||
		preview.Items[1].AmountMinor != 12_800 || preview.AmountMinor != 36_800 {
		t.Fatalf("preview=%+v", preview)
	}
	if preview.SigningDate != "" ||
		preview.SigningDateNoticeCode != "filled_after_authoritative_signature" {
		t.Fatalf("pre-sign date=%q notice=%q", preview.SigningDate, preview.SigningDateNoticeCode)
	}
}

func TestBuildPreviewVariablesFailsClosedOnMissingLanguageNameOrAmountMismatch(t *testing.T) {
	base := &OrderSnapshot{
		OrderNo: "ord_contract_preview_2", CurrencyCode: "USD", AmountMinor: 100,
		RequiresContract: true, OrderVersion: 1,
		Template: TemplateSnapshot{
			TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 1,
			Name: "Service contract", LanguageCode: "en-US",
		},
		Merchant: MerchantIdentity{MerchantNo: "merchant-1", LegalName: []byte("Merchant"), Version: 1},
		Customer: VerifiedIdentity{LegalName: []byte("Customer")},
		Items: []OrderItem{{
			OrderItemNo: "item-1", Quantity: 1, PurchaseCycles: 1,
			PeriodUnitCode: "month", PeriodCount: 1, AmountMinor: 99,
		}},
	}
	if _, err := base.BuildPreviewVariables("contract-preview-2"); err == nil {
		t.Fatal("missing template-language product name and mismatched amount were accepted")
	}
}

func TestBuildPreviewVariablesAllowsManualOptionalSnapshot(t *testing.T) {
	snapshot := &OrderSnapshot{
		OrderNo: "ord_optional_preview_1", CurrencyCode: "CNY", AmountMinor: 100,
		RequiresContract: false, OrderVersion: 2,
		Template: TemplateSnapshot{
			TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 1,
			Name: "Optional service contract", LanguageCode: "en-US",
			ScopeCode: "all_customers", SigningOrderCode: "customer_first",
		},
		Merchant: MerchantIdentity{
			MerchantNo: "merchant-1", LegalName: []byte("Merchant"), Version: 1,
		},
		Customer: VerifiedIdentity{LegalName: []byte("Customer")},
		Items: []OrderItem{{
			OrderItemNo: "item-1", ContractProductName: "Dedicated Server",
			Quantity: 1, PurchaseCycles: 1, PeriodUnitCode: "month",
			PeriodCount: 1, AmountMinor: 100,
		}},
	}
	preview, err := snapshot.BuildPreviewVariables("contract-optional-preview-1")
	if err != nil {
		t.Fatalf("build optional preview variables: %v", err)
	}
	defer preview.Clear()
	if snapshot.RequiresContract || len(preview.Items) != 1 ||
		preview.Items[0].ServiceName != "Dedicated Server" {
		t.Fatalf("optional preview changed requirement or line: snapshot=%+v preview=%+v", snapshot, preview)
	}
}
